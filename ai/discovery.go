package ai

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const discoveryName = "discover_available_tools"
const invocationName = "invoke_available_tool"

// toolDiscovery exposes descriptions, not authority. Execution always goes through
// executeTool, which rechecks the live permission and plugin configuration.
type toolDiscovery struct {
	catalog []*ToolMeta
	allowed map[string]*ToolMeta
	order   []*ToolMeta
	exposed map[string]bool
	calls   int
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
	for name := range d.allowed {
		if !d.exposed[name] {
			return true
		}
	}
	return false
}
func (d *toolDiscovery) prompt() string {
	var b strings.Builder
	b.WriteString("\n【可发现的能力目录】\n下列目录来自程序。根据当前请求和对话语义选择能力，不要求用户说出命令或关键词。需要未加载能力时先调用discover_available_tools读取参数定义及依赖，再用invoke_available_tool执行；arguments必须是符合该工具定义的JSON对象字符串。目录不是授权，部分能力可能对当前用户或群不可用。加载本身不执行操作。无关闲聊不必调用。不得猜测缺少的对象或参数。\n")
	catalog := append([]*ToolMeta(nil), d.catalog...)
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].Name < catalog[j].Name })
	for _, t := range catalog {
		if t.Name != discoveryName && t.Name != invocationName {
			data, _ := json.Marshal(map[string]string{"name": t.Name, "description": t.Description})
			b.Write(data)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func (d *toolDiscovery) schemas() []openai.Tool {
	return []openai.Tool{
		(&ToolMeta{Name: discoveryName, Description: "读取1至4个能力的参数定义及依赖；不会执行业务操作，每轮最多两次。", Params: []Param{{Name: "names", Type: "array", ItemsType: "string", Required: true, Description: "目录中的准确工具名称，最多4个"}}}).Schema(),
		(&ToolMeta{Name: invocationName, Description: "执行本轮已加载的能力。参数必须符合已返回的定义，执行前仍检查当前权限。", Params: []Param{{Name: "name", Type: "string", Required: true, Description: "已加载工具名称"}, {Name: "arguments", Type: "string", Required: true, Description: "业务工具参数，编码为JSON对象字符串"}}}).Schema(),
	}
}
func (d *toolDiscovery) loadedContext() string {
	var schemas []openai.Tool
	for _, t := range d.order {
		if d.exposed[t.Name] {
			schemas = append(schemas, t.Schema())
		}
	}
	data, _ := json.Marshal(schemas)
	return "\n【本轮已加载的工具参数定义】\n使用invoke_available_tool调用；其他能力先发现。旧轮次的加载状态不代表当前授权。\n" + string(data)
}
func (d *toolDiscovery) invoke(call openai.ToolCall, execute func(openai.ToolCall) ToolResult) ToolResult {
	var args struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&args) != nil {
		return rejectedTool("调用参数无效")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || d.allowed[args.Name] == nil || !d.exposed[args.Name] {
		return rejectedTool("请先发现当前可用的工具，不能执行未加载或未授权的能力")
	}
	call.Function = openai.FunctionCall{Name: args.Name, Arguments: args.Arguments}
	return execute(call)
}
func (d *toolDiscovery) discover(raw string) ToolResult {
	d.calls++
	var args struct {
		Names []string `json:"names"`
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if d.calls > 2 || decoder.Decode(&args) != nil || len(args.Names) == 0 || len(args.Names) > 4 {
		return rejectedTool("请一次选择1至4个目录内的工具；本轮最多发现两次")
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
	for name := range seen {
		d.exposed[name] = true
	}
	return ToolResult{Status: ToolSucceeded, Content: "尚未执行任何业务操作。" + d.loadedContext()}
}
