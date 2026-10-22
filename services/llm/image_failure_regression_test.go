package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestCodexImageFailureDoesNotInventQuota(t *testing.T) {
	for _, code := range []string{"generation_failed", "rateLimitExceeded", "usageLimitExceeded"} {
		t.Run(code, func(t *testing.T) {
			home := t.TempDir()
			if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			failure := map[string]any{"code": code, "message": "private provider detail"}
			event, _ := json.Marshal(map[string]any{"method": "item/completed", "params": map[string]any{"item": map[string]any{"type": "imageGeneration", "status": "failed", "failure": failure}}})
			script := "#!/bin/sh\nread -r line\necho '{\"id\":1,\"result\":{}}'\nread -r line\nread -r line\necho '{\"id\":2,\"result\":{\"thread\":{\"id\":\"t\"}}}'\nread -r line\n"
			script += "echo '" + string(event) + "'\n"
			binary := filepath.Join(t.TempDir(), "codex")
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("YUELING_CODEX_BINARY", binary)
			var observed error
			ctx := WithTimingObserver(context.Background(), func(stage string, _ time.Time, err error) {
				if stage == "image_request" {
					observed = err
				}
			})
			data, err := New(Settings{Kind: "codex", Model: "fixture", CodexHome: home, Timeout: 2 * time.Second}).Image(ctx, "只换泳装配色", cutoutPNG())
			if err == nil || len(data) != 0 {
				t.Fatal("provider failure was accepted")
			}
			if observed == nil || !strings.Contains(observed.Error(), "image_status=failed") || !strings.Contains(observed.Error(), "image_failure_present=true") {
				t.Fatalf("image failure lost its diagnostic before process exit: %v", observed)
			}
			message := UserMessage(err)
			if code != "usageLimitExceeded" && strings.Contains(message, "额度") {
				t.Errorf("unknown failure or rate limiting was reported as exhausted quota: %s", message)
			}
			if code == "usageLimitExceeded" && !strings.Contains(message, "额度") {
				t.Error("explicit quota error was lost")
			}
			if strings.Contains(err.Error()+message, "private provider detail") {
				t.Error("raw provider body leaked")
			}
		})
	}
}

func TestTransparentOutputRejectsTranslucentBackground(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 254})
		}
	}
	img.SetNRGBA(1, 1, color.NRGBA{R: 100, A: 255})
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	data, err := checkedImageResult(context.Background(), out.Bytes(), ImageOptions{TransparentBackground: true})
	if err == nil || len(data) != 0 {
		t.Fatal("a painted translucent background was accepted as a transparent cutout")
	}
	if got := classify(err).Detail; got != "transparency_missing" {
		t.Errorf("unexpected validation failure %s", got)
	}
}

func TestFreeformFailureMessageIsNotQuotaEvidence(t *testing.T) {
	for _, raw := range []string{`{"message":"usageLimitExceeded"}`, `{"code":"generation_failed","message":"quota limit unknown"}`} {
		err := codexImageFailure(json.RawMessage(raw))
		if classify(err).Kind == Busy || strings.Contains(UserMessage(err), "额度") {
			t.Error("freeform provider text became quota evidence")
		}
	}
}

func TestCodexImageFailureDiagnosticsDoNotClaimProviderOmission(t *testing.T) {
	events := `{"id":1,"result":{}}
{"id":2,"result":{"thread":{"id":"t"}}}
{"method":"item/completed","params":{"item":{"type":"imageGeneration","status":"failed","result":"private provider detail","failure":null}}}
`
	var data []byte
	_, err := codexExchangeResult(context.Background(), &bytes.Buffer{}, strings.NewReader(events), openai.ChatCompletionRequest{Model: "fixture"}, "/empty", &data)
	if err == nil {
		t.Fatal("failed image was accepted")
	}
	observed := err
	if !strings.Contains(observed.Error(), "image_status=failed") || !strings.Contains(observed.Error(), "image_result_bytes=23") || !strings.Contains(observed.Error(), "image_failure_present=false") {
		t.Fatalf("missing safe protocol metadata: %v", observed)
	}
	if strings.Contains(observed.Error(), "private provider detail") {
		t.Fatal("raw image result leaked")
	}
	if strings.Contains(UserMessage(err), "服务未提供") {
		t.Fatal("missing diagnostics were asserted to be absent upstream")
	}
}
