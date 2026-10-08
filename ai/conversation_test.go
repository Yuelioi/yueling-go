package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestConversationRecovery(t *testing.T) {
	for _, scenario := range []string{"transient", "protocol_text", "tool_narration", "truncated_tool"} {
		t.Run(scenario, func(t *testing.T) {
			oldConfig, oldClient := config.C, _client
			t.Cleanup(func() { config.C = oldConfig; _client = oldClient })
			requests, executed := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				var request openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				if requests == 1 {
					switch scenario {
					case "transient":
						w.WriteHeader(503)
						fmt.Fprint(w, `{"error":{"message":"busy","type":"server_error"}}`)
					case "protocol_text":
						fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"<tool_call>{\"name\":\"summarize_chat\",\"arguments\":{}}</tool_call>"},"finish_reason":"stop"}]}`)
					case "tool_narration":
						fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"我需要先进行 tool call 来获取聊天记录。"},"finish_reason":"stop"}]}`)
					case "truncated_tool":
						fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"summarize_chat","arguments":"{"}}]},"finish_reason":"length"}]}`)
					}
					return
				}
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"大家讨论了发布计划。"},"finish_reason":"stop"}]}`)
			}))
			defer server.Close()
			config.C.AI.Model = "test"
			_client = NewClient("test", server.URL)
			session := newSession(1, 2)
			reply, err := runConversation(context.Background(), session, "总结群聊", "请总结", nil, func(openai.ToolCall) ToolResult {
				executed++
				return ToolResult{Status: ToolReported, Content: "记录", Invoked: true, PossibleSideEffects: true}
			})
			if err != nil || reply != "大家讨论了发布计划。" {
				t.Errorf("reply=%q err=%v", reply, err)
			}
			if executed != 0 {
				t.Errorf("executed %d invalid tool calls", executed)
			}
			for _, msg := range session.Messages {
				if strings.Contains(msg.Content, "<tool_call>") {
					t.Error("protocol text persisted")
				}
			}
		})
	}
}

func TestConversationDoesNotReplayActionsOnRetry(t *testing.T) {
	oldConfig, oldClient := config.C, _client
	t.Cleanup(func() { config.C = oldConfig; _client = oldClient })
	requests, executed := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch requests {
		case 1:
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","reasoning_content":"private reasoning","tool_calls":[{"id":"call_1","type":"function","function":{"name":"action","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
		case 2:
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":{"message":"busy"}}`)
		default:
			if request.Messages[len(request.Messages)-1].ToolCallID != "call_1" {
				t.Error("lost tool result")
			}
			if request.Messages[len(request.Messages)-2].ReasoningContent != "private reasoning" {
				t.Error("lost in-turn reasoning required by reasoning models")
			}
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"操作完成"},"finish_reason":"stop"}]}`)
		}
	}))
	defer server.Close()
	config.C.AI.Model = "test"
	_client = NewClient("test", server.URL)
	session := newSession(1, 2)
	reply, _ := runConversation(context.Background(), session, "执行操作", "system", nil, func(openai.ToolCall) ToolResult {
		executed++
		return ToolResult{Status: ToolReported, Content: "已执行", Invoked: true, PossibleSideEffects: true}
	})
	if reply != "操作完成" || executed != 1 || requests != 3 {
		t.Fatalf("reply=%q executed=%d requests=%d", reply, executed, requests)
	}
	if len(session.Messages) != 4 || session.Messages[1].ReasoningContent != "private reasoning" || session.Messages[2].ToolCallID != "call_1" {
		t.Fatal("lost protocol transcript needed for follow-up requests")
	}
}

func TestConversationRejectsPersistentProtocolAndAuthErrors(t *testing.T) {
	for _, status := range []int{200, 401} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			oldConfig, oldClient := config.C, _client
			t.Cleanup(func() { config.C = oldConfig; _client = oldClient })
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status == 401 {
					fmt.Fprint(w, `{"error":{"message":"secret-provider-body"}}`)
					return
				}
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"<tool_call>broken</tool_call>"},"finish_reason":"stop"}]}`)
			}))
			defer server.Close()
			config.C.AI.Model = "test"
			_client = NewClient("test", server.URL)
			session := newSession(1, 2)
			reply, _ := runConversation(context.Background(), session, "总结群聊", "system", nil, func(openai.ToolCall) ToolResult {
				t.Fatal("unexpected execution")
				return ToolResult{Status: ToolReported, Content: "", Invoked: true, PossibleSideEffects: true}
			})
			if strings.Contains(reply, "tool_call") || strings.Contains(reply, "secret-provider-body") || len(session.Messages) != 0 {
				t.Fatalf("unsafe reply/history: %q %+v", reply, session.Messages)
			}
			wantRequests := 2
			if status == 401 {
				wantRequests = 1
				if !strings.Contains(reply, "认证失败") {
					t.Error(reply)
				}
			}
			if requests != wantRequests {
				t.Fatalf("requests=%d want=%d", requests, wantRequests)
			}
		})
	}
}

func TestTrimHistoryKeepsCompleteTurns(t *testing.T) {
	session := newSession(1, 2)
	session.pushUser(strings.Repeat("旧", 25000))
	session.pushAssistant(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{ID: "old"}}})
	session.pushToolResult("old", "结果")
	session.pushUser("最新问题")
	session.pushAssistant(openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "最新回答"})
	session.trimHistory()
	if len(session.Messages) != 2 || session.Messages[0].Content != "最新问题" {
		t.Fatalf("history=%+v", session.Messages)
	}
}

func TestToolProtocolExplanationIsAllowed(t *testing.T) {
	if leaksToolNarration("tool call 是模型请求执行函数的结构化消息", "解释工具调用怎么实现") {
		t.Fatal("blocked requested technical explanation")
	}
	if !leaksToolNarration("我要先发起 tool call", "总结群聊") {
		t.Fatal("leaked unsolicited tool narration")
	}
}

func TestConversationRetainsReasoningAcrossTurns(t *testing.T) {
	oldConfig, oldClient := config.C, _client
	t.Cleanup(func() { config.C = oldConfig; _client = oldClient })
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		for _, msg := range req.Messages {
			if msg.Role == openai.ChatMessageRoleAssistant && msg.ReasoningContent == "" {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"error":{"message":"Missing reasoning_content in assistant message","type":"invalid_request_error"}}`)
				return
			}
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"已按记录整理。","reasoning_content":"synthetic protocol fixture"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	config.C.AI.Model = "test"
	_client = NewClient("test", server.URL)
	session := newSession(1, 2)
	tool := (&ToolMeta{Name: "summarize_chat"}).Schema()
	for i := 0; i < 2; i++ {
		reply, err := runConversation(context.Background(), session, "总结群聊", "system", []openai.Tool{tool}, func(openai.ToolCall) ToolResult {
			return ToolResult{Status: ToolReported, Content: "data", Invoked: true, PossibleSideEffects: true}
		})
		if err != nil || reply != "已按记录整理。" {
			t.Fatalf("turn=%d reply=%q err=%v", i, reply, err)
		}
	}
}

func TestSessionWaitIsCancelable(t *testing.T) {
	session := newSession(1, 2)
	if err := session.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := session.acquire(ctx); err != context.Canceled {
		t.Fatalf("err=%v", err)
	}
	session.release()
	if err := session.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	session.release()
}

func TestConversationDoesNotStartActionsOnFinalStep(t *testing.T) {
	oldConfig, oldClient := config.C, _client
	t.Cleanup(func() { config.C = oldConfig; _client = oldClient })
	requests, actions := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if requests == maxSteps && req.ToolChoice != "none" {
			t.Error("final step did not request synthesis only")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","reasoning_content":"fixture","tool_calls":[{"id":"call_%d","type":"function","function":{"name":"action","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`, requests)
	}))
	defer server.Close()
	config.C.AI.Model = "test"
	_client = NewClient("test", server.URL)
	_, err := runConversation(context.Background(), newSession(1, 2), "test", "system", []openai.Tool{(&ToolMeta{Name: "action"}).Schema()}, func(openai.ToolCall) ToolResult {
		actions++
		return ToolResult{Status: ToolReported, Content: "done", Invoked: true, PossibleSideEffects: true}
	})
	if err == nil || requests != maxSteps || actions != maxSteps-1 {
		t.Fatalf("err=%v requests=%d actions=%d", err, requests, actions)
	}
}
