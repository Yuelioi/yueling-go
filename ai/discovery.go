package ai

import (
	"encoding/json"
	"io"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const discoveryName = "discover_available_tools"

// toolDiscovery exposes descriptions, not authority. Execution always goes through
// executeTool, which rechecks the live permission and plugin configuration.
type toolDiscovery struct {
	allowed map[string]*ToolMeta
	order   []*ToolMeta
	exposed map[string]bool
	calls   int
}

func newToolDiscovery(allowed []*ToolMeta, exposed map[string]bool) *toolDiscovery {
	d := &toolDiscovery{allowed: map[string]*ToolMeta{}, order: allowed, exposed: exposed}
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
	if !d.pending() {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【可发现的能力目录】\n下列目录来自程序。根据当前请求和对话语义选择能力，不要求用户说出命令或关键词。需要未加载能力时先调用discover_available_tools加载对应名称及依赖，再执行业务工具。加载本身不执行操作。无关闲聊不必调用。不得猜测缺少的对象或参数。\n")
	for _, t := range d.order {
		if d.allowed[t.Name] != nil {
			data, _ := json.Marshal(map[string]string{"name": t.Name, "description": t.Description})
			b.Write(data)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func (d *toolDiscovery) schemas() []openai.Tool {
	var out []openai.Tool
	for _, t := range d.order {
		if d.exposed[t.Name] {
			out = append(out, t.Schema())
		}
	}
	if d.pending() && d.calls < 2 {
		out = append(out, (&ToolMeta{Name: discoveryName, Description: "按能力目录加载1至4个工具；不会执行业务操作。", Params: []Param{{Name: "names", Type: "array", ItemsType: "string", Required: true, Description: "能力目录中的准确工具名称，最多4个"}}}).Schema())
	}
	return out
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
	return ToolResult{Status: ToolSucceeded, Content: "所选工具及允许的依赖已加载。尚未执行任何业务操作，请根据当前任务调用已加载工具。"}
}
