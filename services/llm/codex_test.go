package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestCodexProtocolToolCallReturnsToBot(t *testing.T) {
	events := `{"id":1,"result":{}}
{"id":2,"result":{"thread":{"id":"t"}}}
{"id":3,"result":{}}
{"method":"item/tool/call","id":4,"params":{"threadId":"t","callId":"c","tool":"lookup","arguments":{"query":"hello"}}}
`
	var sent bytes.Buffer
	req := openai.ChatCompletionRequest{Model: "fixture", Messages: []openai.ChatCompletionMessage{{Role: "system", Content: "rules"}, {Role: "user", Content: "hello"}}, Tools: []openai.Tool{{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "lookup", Parameters: map[string]any{"type": "object"}}}}}
	resp, err := codexExchange(context.Background(), &sent, strings.NewReader(events), req, "/empty")
	if err != nil || len(resp.Choices) != 1 || resp.Choices[0].Message.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("tool result: %+v %v", resp, err)
	}
	var requests []map[string]any
	decoder := json.NewDecoder(&sent)
	for decoder.More() {
		var v map[string]any
		if err := decoder.Decode(&v); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, v)
	}
	params := requests[2]["params"].(map[string]any)
	if params["sandbox"] != "read-only" || params["ephemeral"] != true || len(params["environments"].([]any)) != 0 {
		t.Fatal(params)
	}
	if len(requests) != 4 {
		t.Fatal("tool executed/answered in adapter", requests)
	}
	req.ToolChoice = "none"
	if _, err = codexExchange(context.Background(), &bytes.Buffer{}, strings.NewReader(events), req, "/empty"); err == nil {
		t.Fatal("disabled tool was accepted")
	}
}

func TestCodexProtocolRejectsFailedOrUnapprovedTurns(t *testing.T) {
	for _, tail := range []string{
		`{"method":"turn/completed","params":{"turn":{"status":"failed"}}}`,
		`{"method":"item/commandExecution/requestApproval","id":7,"params":{}}`,
		`{"id":2,"error":{"message":"secret"}}`,
	} {
		_, err := codexExchange(context.Background(), &bytes.Buffer{}, strings.NewReader("{\"id\":1,\"result\":{}}\n"+tail), openai.ChatCompletionRequest{Model: "test"}, "/empty")
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	}
}

func TestCodexProtocolFinalContentAndUsage(t *testing.T) {
	events := `{"id":1,"result":{}}
{"id":2,"result":{"thread":{"id":"t"}}}
{"method":"item/completed","params":{"item":{"type":"agentMessage","phase":"commentary","text":"working"}}}
{"method":"thread/tokenUsage/updated","params":{"tokenUsage":{"last":{"inputTokens":100,"cachedInputTokens":40,"outputTokens":10}}}}
{"method":"item/completed","params":{"item":{"type":"agentMessage","phase":"final_answer","text":"done"}}}
{"method":"turn/completed","params":{"turn":{"status":"completed"}}}
`
	resp, err := codexExchange(context.Background(), &bytes.Buffer{}, strings.NewReader(events), openai.ChatCompletionRequest{Model: "test"}, "/empty")
	if err != nil || resp.Choices[0].Message.Content != "done" || resp.Usage.PromptTokensDetails.CachedTokens != 40 {
		t.Fatal(resp, err)
	}
}

func TestCodexAccountQueueCancellation(t *testing.T) {
	home := t.TempDir()
	gate := codexGate(home)
	gate <- struct{}{}
	defer func() { <-gate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&codexBackend{Settings{CodexHome: home}}).CreateChatCompletion(ctx, openai.ChatCompletionRequest{})
	if err != context.Canceled {
		t.Fatal(err)
	}
}

// This opt-in check runs the installed Codex against a local fake inference server.
func TestInstalledCodexWithFixtureProvider(t *testing.T) {
	if os.Getenv("YUELING_TEST_CODEX") != "1" {
		t.Skip("set YUELING_TEST_CODEX=1 to verify the installed app-server")
	}
	binary, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YUELING_CODEX_BINARY", binary)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		calls.Add(1)
		if calls.Load() == 3 {
			data, _ := json.Marshal(request)
			if !bytes.Contains(data, []byte("input_image")) {
				t.Error("image missing from Responses input")
			}
		}
		if calls.Load() == 4 {
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
			return
		}
		if tools, ok := request["tools"].([]any); ok {
			for _, v := range tools {
				tool := v.(map[string]any)
				if tool["name"] != "capability_probe" && tool["name"] != "update_plan" && tool["name"] != "request_user_input" && tool["name"] != "skills" {
					t.Errorf("unexpected native tool: %v", tool["name"])
				}
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		response := map[string]any{"id": "resp_1", "object": "response", "created_at": 1, "status": "completed", "model": "gpt-6.1-sol", "output": []any{map[string]any{"id": "msg_1", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": "fixture OK", "annotations": []any{}}}}}, "usage": map[string]any{"input_tokens": 10, "output_tokens": 3, "total_tokens": 13}}
		if calls.Load() == 2 {
			response["output"] = []any{map[string]any{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "capability_probe", "arguments": "{\"value\":\"OK\"}", "status": "completed"}}
		}
		emit := func(kind string, value any) {
			data, _ := json.Marshal(value)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, data)
		}
		emit("response.created", map[string]any{"type": "response.created", "response": map[string]any{"id": "resp_1", "status": "in_progress", "output": []any{}}})
		item := response["output"].([]any)[0]
		emit("response.output_item.done", map[string]any{"type": "response.output_item.done", "output_index": 0, "item": item})
		emit("response.completed", map[string]any{"type": "response.completed", "response": response})
	}))
	defer server.Close()
	defer server.CloseClientConnections()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"fixture","refresh_token":"fixture","account_id":"fixture","id_token":"fixture"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("model_provider = \"fixture\"\n[model_providers.fixture]\nname = \"fixture\"\nbase_url = %q\nwire_api = \"responses\"\nrequires_openai_auth = false\nsupports_websockets = false\n", server.URL)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	resp, err := (&codexBackend{Settings{CodexHome: home}}).CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: "gpt-6.1-sol", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatalf("installed Codex: %v (inference calls %d)", err, calls.Load())
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "fixture OK" || calls.Load() != 1 {
		t.Fatal(resp, calls.Load())
	}
	req := openai.ChatCompletionRequest{Model: "gpt-6.1-sol", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "call the probe"}}, Tools: []openai.Tool{{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "capability_probe", Parameters: map[string]any{"type": "object", "properties": map[string]any{"value": map[string]string{"type": "string"}}}}}}}
	resp, err = (&codexBackend{Settings{CodexHome: home}}).CreateChatCompletion(ctx, req)
	if err != nil || len(resp.Choices) != 1 || len(resp.Choices[0].Message.ToolCalls) != 1 || resp.Choices[0].Message.ToolCalls[0].Function.Name != "capability_probe" {
		t.Fatalf("dynamic tools: %+v %v", resp, err)
	}

	req.Tools = nil
	req.Messages = []openai.ChatCompletionMessage{{Role: "user", MultiContent: []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeText, Text: "describe the square"}, {Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(fixturePNG())}}}}}
	resp, err = (&codexBackend{Settings{CodexHome: home}}).CreateChatCompletion(ctx, req)
	if err != nil || len(resp.Choices) != 1 || calls.Load() != 3 {
		t.Fatalf("image input: %+v %v", resp, err)
	}
	canceled, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	_, err = (&codexBackend{Settings{CodexHome: home}}).CreateChatCompletion(canceled, req)
	if err != context.DeadlineExceeded {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestCodexImportRejectsActiveConnection(t *testing.T) {
	dir := t.TempDir()
	home := CodexHome(dir, "account")
	gate := codexGate(home)
	gate <- struct{}{}
	defer func() { <-gate }()
	if err := ImportCodexAuth(dir, "account", nil, true); err == nil {
		t.Fatal("import raced an active request")
	}
}
func TestCodexErrorCodesDoNotExposeProviderDetails(t *testing.T) {
	for code, want := range map[string]Kind{"unauthorized": Authentication, "usageLimitExceeded": Busy, "badRequest": Compatibility} {
		err := codexTurnError(json.RawMessage(`{"message":"private detail","codexErrorInfo":"` + code + `"}`))
		if classify(err).Kind != want || strings.Contains(err.Error(), "private") {
			t.Fatal(err)
		}
	}
}

func TestCodexSubscriptionModelRejectionIsActionable(t *testing.T) {
	provider, _ := json.Marshal(map[string]any{"status": 400, "error": map[string]string{"type": "invalid_request_error", "message": "The 'private-model' model is not supported when using Codex with a ChatGPT account."}})
	raw, _ := json.Marshal(map[string]any{"message": string(provider), "codexErrorInfo": "other"})
	err := codexTurnError(raw)
	if classify(err).Kind != Compatibility || !strings.Contains(UserMessage(err), "订阅不支持") || strings.Contains(err.Error(), "private-model") {
		t.Fatalf("model rejection lost or exposed provider details: %v / %s", err, UserMessage(err))
	}
}

func TestCodexProxyIsScopedToChildProcess(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("requires POSIX shell")
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "codex")
	script := `#!/bin/sh
[ "$HTTPS_PROXY" = "http://codex.invalid:7890" ] || exit 10
[ "$HTTP_PROXY" = "http://codex.invalid:7890" ] || exit 11
[ -z "$PGPASSWORD" ] || exit 12
read -r initialize
printf '%s\n' '{"id":1,"result":{}}'
read -r initialized
read -r thread
printf '%s\n' '{"id":2,"result":{"thread":{"id":"t"}}}'
read -r turn
printf '%s\n' '{"id":3,"result":{}}' '{"method":"item/completed","params":{"item":{"type":"agentMessage","phase":"final_answer","text":"OK"}}}' '{"method":"turn/completed","params":{"turn":{"status":"completed"}}}'
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YUELING_CODEX_BINARY", binary)
	t.Setenv("YUELING_CODEX_PROXY", "http://codex.invalid:7890")
	t.Setenv("HTTPS_PROXY", "http://parent.invalid:7890")
	t.Setenv("HTTP_PROXY", "http://parent.invalid:7890")
	t.Setenv("PGPASSWORD", "unrelated-private-secret")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := (&codexBackend{Settings{CodexHome: home}}).CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: "fixture"})
	if err != nil || len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "OK" {
		t.Fatalf("child proxy not applied: %v", err)
	}
	if os.Getenv("HTTPS_PROXY") != "http://parent.invalid:7890" {
		t.Fatal("changed parent proxy")
	}
}
