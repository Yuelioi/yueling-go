package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func fixturePNG() []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	return b.Bytes()
}
func TestImageGenerateAndEditRequests(t *testing.T) {
	source := fixturePNG()
	for _, edit := range []bool{false, true} {
		t.Run(map[bool]string{false: "generate", true: "edit"}[edit], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer private" {
					t.Error("credential missing")
				}
				if edit {
					if r.URL.Path != "/images/edits" {
						t.Error(r.URL.Path)
					}
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Fatal(err)
					}
					f, _, err := r.FormFile("image")
					if err != nil {
						t.Fatal(err)
					}
					f.Close()
					if r.FormValue("model") != "drawing" || r.FormValue("prompt") != "blue square" {
						t.Error("edit fields")
					}
				} else {
					if r.URL.Path != "/images/generations" {
						t.Error(r.URL.Path)
					}
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["model"] != "drawing" || body["prompt"] != "blue square" || body["response_format"] != nil {
						t.Error(body)
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(source)}}})
			}))
			defer srv.Close()
			var input []byte
			if edit {
				input = source
			}
			data, err := New(Settings{Kind: "openai", Model: "drawing", APIKey: "private", BaseURL: srv.URL}).Image(context.Background(), "blue square", input)
			if err != nil || !bytes.Equal(data, source) {
				t.Fatal(err)
			}
		})
	}
}
func TestImageFailureDoesNotRetryOrLeakBody(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(503)
		_, _ = w.Write([]byte("private provider diagnostic"))
	}))
	defer srv.Close()
	_, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).Image(context.Background(), "test", nil)
	if err == nil || calls.Load() != 1 || err.Error() == "private provider diagnostic" {
		t.Fatal(err, calls.Load())
	}
}
