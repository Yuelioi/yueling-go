package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func referencePNG(t *testing.T, red uint8) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: red, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestImageEditSendsAllReferencesInOrder(t *testing.T) {
	for _, count := range []int{2, 3} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			sources := [][]byte{referencePNG(t, 50), referencePNG(t, 100), referencePNG(t, 150)}[:count]
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/images/edits" || r.ParseMultipartForm(1<<20) != nil {
					t.Error("multi-reference request must use image editing")
					return
				}
				defer r.MultipartForm.RemoveAll()
				files := r.MultipartForm.File["image[]"]
				if len(files) != count || len(r.MultipartForm.File["image"]) != 0 {
					t.Errorf("references missing: %d want %d", len(files), count)
					return
				}
				for i, file := range files {
					f, err := file.Open()
					if err != nil {
						t.Error(err)
						return
					}
					data, _ := io.ReadAll(f)
					f.Close()
					if !bytes.Equal(data, sources[i]) {
						t.Errorf("reference %d reordered or replaced", i+1)
					}
				}
				if r.FormValue("prompt") != "参考图1取面部，参考图2及3取服装" {
					t.Error("lost reference roles")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(sources[0])}}})
			}))
			defer srv.Close()
			got, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).EditImages(context.Background(), "参考图1取面部，参考图2及3取服装", sources)
			if err != nil || !bytes.Equal(got, sources[0]) || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestImageReferenceValidationBeforeModelCalls(t *testing.T) {
	valid, animation := referencePNG(t, 100), animationFixture(t, 2)
	for _, tc := range []struct {
		name, detail string
		sources      [][]byte
	}{
		{"missing", "image_reference_limit", nil},
		{"over limit", "image_reference_limit", [][]byte{valid, valid, valid, valid}},
		{"invalid second", "invalid_source_image", [][]byte{valid, []byte("invalid")}},
		{"invalid after animation", "invalid_source_image", [][]byte{animation, nil}},
		{"multiple animations", "multiple_animated_references", [][]byte{animation, valid, animation}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
			defer srv.Close()
			_, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).EditImages(context.Background(), "修改图片", tc.sources)
			if err == nil || classify(err).Detail != tc.detail || calls != 0 {
				t.Fatalf("invalid batch reached provider: detail=%s calls=%d err=%v", classify(err).Detail, calls, err)
			}
		})
	}
}

func TestCodexReceivesAllImageReferences(t *testing.T) {
	sources := [][]byte{referencePNG(t, 30), referencePNG(t, 60), referencePNG(t, 90)}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "request.json")
	event, _ := json.Marshal(map[string]any{"method": "item/completed", "params": map[string]any{"item": map[string]any{"type": "imageGeneration", "status": "completed", "result": base64.StdEncoding.EncodeToString(sources[0])}}})
	script := "#!/bin/sh\nread -r line\necho '{\"id\":1,\"result\":{}}'\nread -r line\nread -r line\necho '{\"id\":2,\"result\":{\"thread\":{\"id\":\"t\"}}}'\nread -r line\n"
	script += fmt.Sprintf("printf '%%s\\n' \"$line\" > %q\n", capture)
	script += "echo '" + string(event) + "'\necho '{\"method\":\"turn/completed\",\"params\":{\"turn\":{\"status\":\"completed\"}}}'\n"
	binary := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YUELING_CODEX_BINARY", binary)
	_, err := New(Settings{Kind: "codex", Model: "fixture", CodexHome: home, Timeout: 3 * time.Second}).EditImages(context.Background(), "第一张头像，后两张服装三视图", sources)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		Method string `json:"method"`
		Params struct {
			Input []struct{ Type, URL, Text string } `json:"input"`
		} `json:"params"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		t.Fatal(err)
	}
	var got [][]byte
	for _, item := range request.Params.Input {
		if item.Type == "image" {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(item.URL, "data:image/png;base64,"))
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, decoded)
		}
	}
	if request.Method != "turn/start" || !reflect.DeepEqual(got, sources) || !strings.Contains(request.Params.Input[0].Text, "后两张服装三视图") {
		t.Fatal("Codex reference bytes, roles or ordering were lost")
	}
}

func TestAnimatedEditPreservesStaticReferences(t *testing.T) {
	static, animated := referencePNG(t, 123), animationFixture(t, 2)
	sources := [][]byte{static, animated}
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.ParseMultipartForm(1<<20) != nil {
			t.Error("frame reference form missing")
			return
		}
		defer r.MultipartForm.RemoveAll()
		files := r.MultipartForm.File["image[]"]
		if len(files) != 2 || !strings.Contains(r.FormValue("prompt"), "第2张是同一GIF") {
			t.Error("frame role or static reference lost")
			return
		}
		for i, file := range files {
			f, _ := file.Open()
			data, _ := io.ReadAll(f)
			f.Close()
			if i == 0 && !bytes.Equal(data, static) {
				t.Error("static reference changed between frames")
			}
			if i == 1 {
				if _, err := png.Decode(bytes.NewReader(data)); err != nil {
					t.Error("animation frame was not normalized to PNG")
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(static)}}})
	}))
	defer srv.Close()
	got, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).EditImages(context.Background(), "参考头像换成这个角色，保留动画", sources)
	if err != nil || calls.Load() != 2 || !bytes.Equal(sources[1], animated) {
		t.Fatalf("calls=%d input_mutated=%v err=%v", calls.Load(), !bytes.Equal(sources[1], animated), err)
	}
	animation, err := gif.DecodeAll(bytes.NewReader(got))
	if err != nil || len(animation.Image) != 2 || animation.LoopCount != 3 {
		t.Fatal("reference edit flattened animation", err)
	}
}
