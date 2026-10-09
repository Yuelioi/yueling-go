package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestCanceledZssmDoesNotSendModelRequests(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	vl := config.VLConfig{Key: "fixture", BaseURL: server.URL, Model: "fixture"}
	if _, err := describeImageDataURL(ctx, vl, "data:image/png;base64,fixture"); err == nil {
		t.Fatal("canceled vision task succeeded")
	}
	if _, err := zssmGenerate(ctx, config.AIConfig{DeepSeekKey: "fixture", BaseURL: server.URL, Model: "fixture"}, "system", "user"); err == nil {
		t.Fatal("canceled text task succeeded")
	}
	if calls.Load() != 0 {
		t.Fatal("canceled task sent model requests")
	}
}

func TestVisionRequestUsesSelectedSharedOrSeparateProfile(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	endpoint := func(model, key string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer "+key {
				t.Error("incorrect vision endpoint or key")
			}
			var body struct {
				Model    string `json:"model"`
				Messages []struct {
					Content []struct {
						Type     string `json:"type"`
						ImageURL struct {
							URL string `json:"url"`
						} `json:"image_url"`
					} `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Model != model || len(body.Messages) != 1 || len(body.Messages[0].Content) != 2 || body.Messages[0].Content[0].Type != "image_url" || body.Messages[0].Content[0].ImageURL.URL != "data:image/png;base64,fixture" {
				t.Error("vision model or multimodal payload incorrect")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"image description"}}]}`))
		}))
	}
	shared := endpoint("shared-model", "shared-key")
	defer shared.Close()
	separate := endpoint("vision-model", "vision-key")
	defer separate.Close()
	c := old
	c.AI.Profiles = []config.AIProfile{{ID: "shared", Name: "shared", Model: "shared-model", APIKey: "shared-key", BaseURL: shared.URL, Vision: true}, {ID: "vision", Name: "vision", Model: "vision-model", APIKey: "vision-key", BaseURL: separate.URL, Vision: true}}
	c.AI.ConversationProfile = "shared"
	for _, id := range []string{"shared", "vision"} {
		c.AI.VisionProfile = id
		config.Publish(c)
		description, err := describeImageDataURL(context.Background(), config.Current().AI.VL, "data:image/png;base64,fixture")
		if err != nil || description != "image description" {
			t.Fatalf("vision request: %v", err)
		}
	}
}
