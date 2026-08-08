package bot

import (
	"encoding/json"
	"testing"
	"time"
)

func captureBotAction(t *testing.T, invoke func(*BotAPI) error) (string, map[string]any) {
	t.Helper()

	sendCh := make(chan []byte, 1)
	done := make(chan struct{})
	api := &BotAPI{sendCh: sendCh, done: done}
	errCh := make(chan error, 1)
	go func() {
		errCh <- invoke(api)
	}()

	var payload []byte
	select {
	case payload = <-sendCh:
	case <-time.After(time.Second):
		t.Fatal("BotAPI action was not sent")
	}

	var request struct {
		Action string         `json:"action"`
		Params map[string]any `json:"params"`
		Echo   string         `json:"echo"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	api.deliver(request.Echo, json.RawMessage(`{}`))

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("BotAPI action error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("BotAPI action did not finish")
	}

	return request.Action, request.Params
}

func TestQQGroupActionsUseExpectedOneBotCalls(t *testing.T) {
	tests := []struct {
		name       string
		invoke     func(*BotAPI) error
		wantAction string
		wantParams map[string]any
	}{
		{
			name: "set special title",
			invoke: func(api *BotAPI) error {
				return api.SetGroupSpecialTitle(100, 200, "摸鱼冠军")
			},
			wantAction: "set_group_special_title",
			wantParams: map[string]any{"group_id": float64(100), "user_id": float64(200), "special_title": "摸鱼冠军"},
		},
		{
			name: "delete essence",
			invoke: func(api *BotAPI) error {
				return api.DeleteEssenceMsg(300)
			},
			wantAction: "delete_essence_msg",
			wantParams: map[string]any{"message_id": float64(300)},
		},
		{
			name: "group poke",
			invoke: func(api *BotAPI) error {
				return api.GroupPoke(100, 200)
			},
			wantAction: "group_poke",
			wantParams: map[string]any{"group_id": float64(100), "user_id": float64(200)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, params := captureBotAction(t, tt.invoke)
			if action != tt.wantAction {
				t.Fatalf("action = %q, want %q", action, tt.wantAction)
			}
			if got, want := mustJSON(t, params), mustJSON(t, tt.wantParams); got != want {
				t.Fatalf("params = %s, want %s", got, want)
			}
		})
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(raw)
}
