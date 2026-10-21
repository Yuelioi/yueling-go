package ai

import (
	"bytes"
	"encoding/json"
	"github.com/fatih/color"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestImageInvocationAcceptsEquivalentArgumentEncodings(t *testing.T) {
	for _, tool := range []struct {
		name, payload string
		params        []Param
	}{
		{"inspect_image", `{"image_id":"image_13","question":"描述人物姿势"}`, []Param{{Name: "image_id", Type: "string", Required: true}, {Name: "question", Type: "string", Required: true}}},
		{"edit_image", `{"image_ids":["image_13","image_14"],"prompt":"保留长相，参考第二张姿势"}`, []Param{{Name: "image_ids", Type: "array", ItemsType: "string", Required: true}, {Name: "prompt", Type: "string", Required: true}}},
	} {
		for _, encoding := range []string{"string", "object"} {
			t.Run(tool.name+"/"+encoding, func(t *testing.T) {
				meta := &ToolMeta{Name: tool.name, Params: tool.params}
				discovery := newToolDiscovery([]*ToolMeta{meta}, map[string]bool{})
				if result := discovery.discover(`{"names":["` + tool.name + `"]}`); result.Status != ToolSucceeded {
					t.Fatal(result)
				}
				var arguments any = json.RawMessage(tool.payload)
				if encoding == "string" {
					arguments = tool.payload
				}
				outer, err := json.Marshal(map[string]any{"name": tool.name, "arguments": arguments})
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				result := discovery.invoke(openai.ToolCall{ID: "fixture", Function: openai.FunctionCall{Name: invocationName, Arguments: string(outer)}}, func(call openai.ToolCall) ToolResult {
					calls++
					if call.Function.Name != tool.name {
						t.Fatal("wrong tool")
					}
					if _, err := parseToolArguments(meta, call.Function.Arguments); err != nil {
						t.Fatal("valid image parameters were not preserved", err)
					}
					return ToolResult{Status: ToolSucceeded, Invoked: true, Content: "offline fixture"}
				})
				if result.Status != ToolSucceeded || calls != 1 {
					t.Fatalf("valid %s image arguments rejected before handler: status=%s result=%q calls=%d", encoding, result.Status, result.Content, calls)
				}
			})
		}
	}
}

func TestInvocationCompatibilityPreservesValidationAndAuthorization(t *testing.T) {
	meta := &ToolMeta{Name: "inspect_image", Params: []Param{{Name: "image_id", Type: "string", Required: true}, {Name: "question", Type: "string", Required: true}}}
	for _, payload := range []string{
		`{"name":"inspect_image","arguments":null}`,
		`{"name":"inspect_image","arguments":[]}`,
		`{"name":"inspect_image","arguments":true}`,
		`{"name":"inspect_image","arguments":42}`,
		`{"name":"inspect_image"}`,
		`{"name":"inspect_image","arguments":"[]"}`,
		`{"name":"inspect_image","arguments":"null"}`,
		`{"name":"inspect_image","arguments":"{} {}"}`,
		`{"name":"inspect_image","arguments":"{broken"}`,
		`{"name":"inspect_image","arguments":{},"extra":true}`,
		`{"name":"inspect_image","arguments":{}} {}`,
		`{"name":"unknown","arguments":{}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			discovery := newToolDiscovery([]*ToolMeta{meta}, map[string]bool{"inspect_image": true})
			calls := 0
			result := discovery.invoke(openai.ToolCall{Function: openai.FunctionCall{Name: invocationName, Arguments: payload}}, func(openai.ToolCall) ToolResult { calls++; return ToolResult{Status: ToolSucceeded} })
			if result.Status != ToolRejected || result.Invoked || calls != 0 {
				t.Fatal("invalid envelope reached execution", result, calls)
			}
		})
	}
	for _, exposed := range []bool{false, true} {
		discovery := newToolDiscovery([]*ToolMeta{meta}, map[string]bool{"inspect_image": exposed})
		calls := 0
		result := discovery.invoke(openai.ToolCall{Function: openai.FunctionCall{Name: invocationName, Arguments: `{"name":"inspect_image","arguments":{"image_id":"image_13","question":42,"extra":true}}`}}, func(call openai.ToolCall) ToolResult {
			calls++
			if _, err := parseToolArguments(meta, call.Function.Arguments); err != nil {
				return rejectedTool(err.Error())
			}
			t.Fatal("business parameter validation bypassed")
			return ToolResult{}
		})
		if result.Status != ToolRejected || result.Invoked || calls != map[bool]int{false: 0, true: 1}[exposed] {
			t.Fatal("authorization or business validation bypassed", result, calls)
		}
	}
}

func TestInvocationDiagnosticsExcludePrivateParameters(t *testing.T) {
	var logs bytes.Buffer
	old := color.Error
	color.Error = &logs
	t.Cleanup(func() { color.Error = old })
	discovery := newToolDiscovery([]*ToolMeta{{Name: "edit_image"}}, map[string]bool{"edit_image": true})
	result := discovery.invoke(openai.ToolCall{Function: openai.FunctionCall{Name: invocationName, Arguments: `{"name":"edit_image","arguments":{"image_id":"private-image-id","prompt":"private-prompt https://private.invalid/secret?token=private-key"}}`}}, func(openai.ToolCall) ToolResult { return ToolResult{Status: ToolSucceeded} })
	if result.Status != ToolSucceeded {
		t.Fatal(result)
	}
	text := logs.String()
	if !strings.Contains(text, "name=edit_image") || !strings.Contains(text, "representation=object") {
		t.Fatal("missing structural diagnosis")
	}
	for _, secret := range []string{"private-image-id", "private-prompt", "private.invalid", "private-key"} {
		if strings.Contains(text, secret) {
			t.Fatal("diagnostics exposed parameter content")
		}
	}
}
