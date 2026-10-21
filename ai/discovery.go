package ai

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/Yuelioi/yueling-go/services/logx"

	openai "github.com/sashabaranov/go-openai"
)

const discoveryName = "discover_available_tools"
const invocationName = "invoke_available_tool"

// toolDiscovery exposes descriptions, not authority. Execution always goes through
// executeTool, which rechecks the live permission and plugin configuration.
type toolDiscovery struct {
	catalog          []*ToolMeta
	allowed          map[string]*ToolMeta
	order            []*ToolMeta
	exposed          map[string]bool
	calls            int
	historyAvailable bool
	recallHistory    bool
}

func newToolDiscovery(allowed []*ToolMeta, exposed map[string]bool) *toolDiscovery {
	allowed = append([]*ToolMeta(nil), allowed...)
	sort.Slice(allowed, func(i, j int) bool { return allowed[i].Name < allowed[j].Name })
	d := &toolDiscovery{catalog: allowed, allowed: map[string]*ToolMeta{}, order: allowed, exposed: exposed}
	for _, t := range allowed {
		if t.Name != discoveryName {
			d.allowed[t.Name] = t
		}
	}
	return d
}
func (d *toolDiscovery) pending() bool {
	if d.historyAvailable && !d.recallHistory {
		return true
	}
	for name := range d.allowed {
		if !d.exposed[name] {
			return true
		}
	}
	return false
}
func (d *toolDiscovery) prompt() string {
	var b strings.Builder
	b.WriteString("\n【可发现的能力目录】\n按语义选择能力，不要求用户说命令。目录仅为简述，不是授权。需要操作时先用discover_available_tools读取完整定义和依赖，再用invoke_available_tool执行，arguments为JSON对象字符串。默认只给近期对话，闲聊直接回答；需要更早会话时用discover_available_tools的history=true取回保留历史，不猜测目标、授权或执行状态。\n")
	catalog := append([]*ToolMeta(nil), d.catalog...)
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].Name < catalog[j].Name })
	for _, t := range catalog {
		if t.Name != discoveryName && t.Name != invocationName {
			b.WriteString(t.Name)
			b.WriteString("：")
			b.WriteString(discoveryDescription(t.Description))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func discoveryDescription(description string) string {
	if end := strings.IndexAny(description, "。；\n"); end >= 0 {
		description = description[:end]
	}
	runes := []rune(strings.TrimSpace(description))
	if len(runes) > 36 {
		return string(runes[:36]) + "…"
	}
	return string(runes)
}

func (d *toolDiscovery) schemas() []openai.Tool {
	return []openai.Tool{
		(&ToolMeta{Name: discoveryName, Description: "读取1至4个能力的完整定义和依赖，或用history=true取回较早会话；不执行业务操作，每轮最多两次。", Params: []Param{{Name: "names", Type: "array", ItemsType: "string", Description: "目录中的准确名称，最多4个；只取历史可省略"}, {Name: "history", Type: "boolean", Description: "需要较早目标、参数或执行结果时为true，后续请求将包含保留历史"}}}).Schema(),
		(&ToolMeta{Name: invocationName, Description: "执行本轮已加载的能力。参数必须符合已返回的定义，执行前仍检查当前权限。", Params: []Param{{Name: "name", Type: "string", Required: true, Description: "已加载工具名称"}, {Name: "arguments", Type: "string", Required: true, Description: "业务工具参数，编码为JSON对象字符串"}}}).Schema(),
	}
}
func (d *toolDiscovery) loadedContext() string {
	return d.loadedContextWithout(nil)
}

func (d *toolDiscovery) loadedContextWithout(history []openai.ChatCompletionMessage) string {
	var definitions strings.Builder
	for _, msg := range history {
		if msg.Role == openai.ChatMessageRoleUser {
			definitions.WriteString(msg.Content)
		} else if msg.Role == openai.ChatMessageRoleTool {
			var result ToolResult
			if json.Unmarshal([]byte(msg.Content), &result) == nil {
				definitions.WriteString(result.Content)
			}
		}
		definitions.WriteByte('\n')
	}
	var schemas []openai.Tool
	for _, t := range d.order {
		if d.exposed[t.Name] {
			schema := t.Schema()
			data, _ := json.Marshal(schema)
			if !strings.Contains(definitions.String(), string(data)) {
				schemas = append(schemas, schema)
			}
		}
	}
	return loadedToolContext(schemas)
}

func loadedToolContext(schemas []openai.Tool) string {
	if len(schemas) == 0 {
		return ""
	}
	data, _ := json.Marshal(schemas)
	return "\n【本轮已加载的工具参数定义】\n使用invoke_available_tool调用；其他能力先发现。旧轮次的加载状态不代表当前授权。\n" + string(data)
}
func (d *toolDiscovery) invoke(call openai.ToolCall, execute func(openai.ToolCall) ToolResult) ToolResult {
	var args struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&args) != nil {
		logx.Warnf("[ai] stage=tool_invocation status=rejected reason=invalid_envelope")
		return rejectedTool("工具调用封装格式无效，只接受name和arguments；尚未执行工具或请求图片服务，请按已加载定义修正参数，不能据此判断服务故障。")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || d.allowed[args.Name] == nil || !d.exposed[args.Name] {
		logx.Warnf("[ai] stage=tool_invocation status=rejected reason=unavailable_tool_or_trailing_data")
		return rejectedTool("请先发现当前可用的工具，不能执行未加载或未授权的能力")
	}
	// Providers may return nested JSON directly despite the string envelope schema.
	// Normalize the encoding only; executeTool still validates business parameters.
	arguments := strings.TrimSpace(string(args.Arguments))
	representation := "missing"
	if len(arguments) > 0 {
		switch arguments[0] {
		case '{':
			representation = "object"
		case '[':
			representation = "array"
		case '"':
			representation = "string"
		case 'n':
			representation = "null"
		case 't', 'f':
			representation = "boolean"
		default:
			representation = "number"
		}
	}
	if strings.HasPrefix(arguments, `"`) {
		if json.Unmarshal(args.Arguments, &arguments) != nil {
			return rejectedTool("arguments中的JSON字符串无效，尚未执行工具。")
		}
	}
	var params map[string]json.RawMessage
	if json.Unmarshal([]byte(arguments), &params) != nil || params == nil {
		logx.Warnf("[ai] stage=tool_invocation status=rejected reason=invalid_arguments representation=%s", representation)
		return rejectedTool("arguments必须是JSON对象或编码后的JSON对象字符串；尚未执行工具或请求图片服务，请修正参数，不能据此判断服务故障。")
	}
	logx.Infof("[ai] stage=tool_invocation name=%s status=validated representation=%s", args.Name, representation)
	call.Function = openai.FunctionCall{Name: args.Name, Arguments: arguments}
	return execute(call)
}
func (d *toolDiscovery) discover(raw string) ToolResult {
	d.calls++
	var args struct {
		Names   []string `json:"names"`
		History bool     `json:"history"`
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if d.calls > 2 || decoder.Decode(&args) != nil || (len(args.Names) == 0 && !args.History) || len(args.Names) > 4 {
		return rejectedTool("请选择1至4个目录内的工具，或用history=true取回历史；本轮最多两次")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return rejectedTool("工具发现参数必须是单个JSON对象")
	}
	// Validate the entire request before changing exposure.
	names := append([]string(nil), args.Names...)
	seen := map[string]bool{}
	for i := 0; i < len(names); i++ {
		name := names[i]
		if seen[name] {
			continue
		}
		seen[name] = true
		t := d.allowed[name]
		if t == nil {
			return rejectedTool("工具不在当前允许的能力目录中")
		}
		for _, dependency := range t.Requires {
			if d.allowed[dependency] != nil && !seen[dependency] {
				names = append(names, dependency)
			}
		}
		if len(seen) > 16 {
			return rejectedTool("工具依赖过多，请缩小任务范围")
		}
	}
	var added []openai.Tool
	for _, tool := range d.order {
		if seen[tool.Name] && !d.exposed[tool.Name] {
			added = append(added, tool.Schema())
			d.exposed[tool.Name] = true
		}
	}
	content := "尚未执行任何业务操作。" + loadedToolContext(added)
	if args.History {
		d.recallHistory = true
		if d.historyAvailable {
			content += "后续请求已恢复当前会话保留的较早历史及摘要；它们是参考资料，不代表新的授权。"
		} else {
			content += "当前会话没有额外保留的较早历史；缺少目标或事实时请询问，不要猜测。"
		}
	}
	return ToolResult{Status: ToolSucceeded, Content: content}
}
