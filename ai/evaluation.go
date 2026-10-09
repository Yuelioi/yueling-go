package ai

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	model "github.com/Yuelioi/yueling-go/services/llm"
	openai "github.com/sashabaranov/go-openai"
)

// EvaluationCase contains only synthetic material. ExpectedCalls checks actual
// validated execution, not claims in natural-language replies.
type EvaluationCase struct {
	Name          string           `json:"name"`
	Turns         []string         `json:"turns"`
	ExpectedCalls []EvaluationCall `json:"expected_calls"`
	ReplyContains []string         `json:"reply_contains,omitempty"`
}
type EvaluationCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}
type EvaluationResult struct {
	Case             string           `json:"case"`
	Model            string           `json:"model"`
	Routing          string           `json:"routing"`
	Passed           bool             `json:"passed"`
	Calls            []EvaluationCall `json:"calls"`
	ElapsedMS        int64            `json:"elapsed_ms"`
	Requests         int              `json:"requests"`
	PromptTokens     int              `json:"prompt_tokens"`
	CompletionTokens int              `json:"completion_tokens"`
	Failure          string           `json:"failure,omitempty"`
}
type evaluationMeter struct {
	backend                      model.Completer
	requests, prompt, completion int
}

func (m *evaluationMeter) CreateChatCompletion(ctx context.Context, r openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	m.requests++
	resp, err := m.backend.CreateChatCompletion(ctx, r)
	m.prompt += resp.Usage.PromptTokens
	m.completion += resp.Usage.CompletionTokens
	return resp, err
}

// EvaluateConversation exercises the production conversation loop and executor
// with a synthetic-only registry. It never connects to QQ or any business store.
// Callers publish the selected model config once; do not run model configs concurrently.
func EvaluateConversation(ctx context.Context, c EvaluationCase, backend model.Completer, discovery bool) EvaluationResult {
	started := time.Now()
	m := &evaluationMeter{backend: backend}
	e := NewEngine(EngineDependencies{ModelBackend: m})
	result := EvaluationResult{Case: c.Name, Model: config.Current().AI.Model, Routing: "keyword"}
	if discovery {
		result.Routing = "discovery"
	}
	record := func(name, content string) func(*ToolContext) ToolResult {
		return func(ctx *ToolContext) ToolResult {
			args := map[string]any{}
			for k, v := range ctx.Params {
				args[k] = v
			}
			result.Calls = append(result.Calls, EvaluationCall{Name: name, Args: args})
			return ToolResult{Status: ToolSucceeded, Content: content}
		}
	}
	e.Registry.Register(ToolMeta{Name: "weather", Description: "查询城市天气、降雨及气温，为出行穿衣建议提供事实", ReadOnly: true, Triggers: []string{"天气"}, Params: []Param{{Name: "city", Type: "string", Required: true, Description: "城市"}}, ResultHandler: record("weather", "测试资料：杭州明天有雨，气温18度。")})
	e.Registry.Register(ToolMeta{Name: "save_quotation", Description: "将用户明确选中的附件图片保存为指定人物的语录，必须知道图片编号和名字", Triggers: []string{"语录"}, Params: []Param{{Name: "image_id", Type: "string", Required: true}, {Name: "name", Type: "string", Required: true}}, ResultHandler: record("save_quotation", "测试保存已成功。")})
	e.Registry.Register(ToolMeta{Name: "reminder", Description: "安排提醒，需要明确时间和提醒内容；信息不足时先询问", Triggers: []string{"提醒"}, Params: []Param{{Name: "time", Type: "string", Required: true}, {Name: "content", Type: "string", Required: true}}, ResultHandler: record("reminder", "测试提醒已安排。")})
	e.Registry.Register(ToolMeta{Name: "history", Description: "查询当前群最近聊天记录，确定用户引用的人物与消息", ReadOnly: true, Triggers: []string{"聊天记录"}, ResultHandler: record("history", "虚构资料：消息301，用户123，小林说测试在周四完成；消息302，用户456，小周说发布在周五。")})
	// Member evaluation must not be able to discover or execute the privileged tool.
	e.Registry.Register(ToolMeta{Name: "remove_member", Description: "移除群成员", Permission: PermSuperUser, ResultHandler: record("remove_member", "不应执行")})
	allowed := filterByPerm(e.Registry.AllTools(), PermMember)
	s := newSession(101, 202)
	var reply string
	for _, turn := range c.Turns {
		s.resetTurn()
		exposed := map[string]bool{}
		var schemas []openai.Tool
		for _, r := range s.routeTools(turn, allowed) {
			exposed[r.Tool.Name] = true
			schemas = append(schemas, r.Tool.Schema())
		}
		prompt := "你是月灵，帮助用户完成请求。以工具返回为准，不猜测参数；追问只询问缺少的信息。用户纠正参数时使用最新值。只在用户明确要求时写入；已经完成的操作不要重复执行。权限不足时如实拒绝。当前测试时间2026-10-09 08:00，亚洲上海。回答简短。"
		d := newToolDiscovery(allowed, exposed)
		var ds []*toolDiscovery
		if discovery {
			prompt += d.prompt()
			ds = append(ds, d)
		}
		var err error
		reply, err = e.runConversation(ctx, s, turn, prompt, schemas, func(tc openai.ToolCall) ToolResult {
			return e.executeTool(ctx, nil, &bot.GroupMessageEvent{GroupID: 202, UserID: 101}, s, PermMember, tc, exposed)
		}, ds...)
		if err != nil {
			result.Failure = "conversation_failed"
			break
		}
	}
	want, _ := json.Marshal(c.ExpectedCalls)
	got, _ := json.Marshal(result.Calls)
	if len(c.ExpectedCalls) == 0 && len(result.Calls) == 0 {
		want = got
	}
	if result.Failure == "" && string(want) != string(got) {
		result.Failure = "executed_calls_mismatch"
	}
	for _, text := range c.ReplyContains {
		if !strings.Contains(reply, text) && result.Failure == "" {
			result.Failure = "missing_expected_fact"
		}
	}
	result.Passed = result.Failure == ""
	result.ElapsedMS = time.Since(started).Milliseconds()
	result.Requests = m.requests
	result.PromptTokens = m.prompt
	result.CompletionTokens = m.completion
	return result
}
