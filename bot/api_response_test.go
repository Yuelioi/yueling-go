package bot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFailedApprovalResponseReturnsError(t *testing.T) {
	send := make(chan []byte, 1)
	api := &BotAPI{sendCh: send, done: make(chan struct{})}
	result := make(chan error, 1)
	go func() { result <- api.SetGroupAddRequest("request-flag", "add", true, "") }()
	var payload struct {
		Echo string `json:"echo"`
	}
	select {
	case raw := <-send:
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("approval not sent")
	}
	raw, _ := json.Marshal(map[string]any{"status": "failed", "retcode": 1200, "message": "permission denied", "wording": "permission denied", "data": nil, "echo": payload.Echo})
	New().dispatch(api, raw)
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("failed approval was reported as success (nil error)")
		}
		for _, detail := range []string{"set_group_add_request", "retcode=1200", "permission denied"} {
			if !strings.Contains(err.Error(), detail) {
				t.Fatalf("error %q omits %q", err, detail)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("approval did not return")
	}
}

func TestAPIResponseStatusAndData(t *testing.T) {
	for _, tc := range []struct {
		name      string
		response  string
		wantData  string
		wantError string
	}{
		{"success", `{"status":"ok","retcode":0,"data":{"message_id":42}}`, `{"message_id":42}`, ""},
		{"success without data", `{"status":"ok","retcode":0,"data":null}`, `null`, ""},
		{"accepted async", `{"status":"async","retcode":1,"data":null}`, `null`, ""},
		{"failed status", `{"status":"failed","retcode":0,"data":null,"wording":"权限不足"}`, "", "权限不足"},
		{"failed retcode", `{"status":"ok","retcode":1404,"data":null}`, "", "retcode=1404"},
		{"failure details", `{"status":"failed","retcode":1200,"message":"request failed","wording":"权限不足"}`, "", "request failed; 权限不足"},
		{"nested status is payload", `{"status":"ok","retcode":0,"data":{"status":"file_complete"}}`, `{"status":"file_complete"}`, ""},
		{"malformed retcode", `{"status":"ok","retcode":"invalid","data":null}`, "", "decode test_action response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			send := make(chan []byte, 1)
			done := make(chan struct{})
			defer close(done)
			api := &BotAPI{sendCh: send, done: done}
			type result struct {
				data json.RawMessage
				err  error
			}
			results := make(chan result, 1)
			go func() { data, err := api.call("test_action", nil); results <- result{data, err} }()
			var request struct {
				Echo string `json:"echo"`
			}
			select {
			case raw := <-send:
				if err := json.Unmarshal(raw, &request); err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("request not sent")
			}
			response := strings.TrimSuffix(tc.response, "}") + `,"echo":"` + request.Echo + `"}`
			New().dispatch(api, []byte(response))
			select {
			case got := <-results:
				if tc.wantError != "" {
					if got.err == nil || !strings.Contains(got.err.Error(), tc.wantError) || !strings.Contains(got.err.Error(), "test_action") {
						t.Fatalf("error=%v, want %q", got.err, tc.wantError)
					}
				} else if got.err != nil || string(got.data) != tc.wantData {
					t.Fatalf("data=%s err=%v, want %s", got.data, got.err, tc.wantData)
				}
			case <-time.After(time.Second):
				t.Fatal("response did not return")
			}
		})
	}
}
