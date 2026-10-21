package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
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

func TestImageCutoutRequestsTransparency(t *testing.T) {
	for _, edit := range []bool{false, true} {
		t.Run(map[bool]string{false: "generate", true: "edit"}[edit], func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var background, format string
				if edit {
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Error(err)
						return
					}
					defer r.MultipartForm.RemoveAll()
					background, format = r.FormValue("background"), r.FormValue("output_format")
				} else {
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					background, _ = body["background"].(string)
					format, _ = body["output_format"].(string)
				}
				if background != "transparent" || format != "png" {
					t.Errorf("cutout request lost alpha requirement: background=%q output_format=%q", background, format)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(cutoutPNG())}}})
			}))
			defer srv.Close()
			var source []byte
			if edit {
				source = fixturePNG()
			}
			_, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).Image(context.Background(), "抠图，保留主体，输出真正透明底PNG", source, ImageOptions{TransparentBackground: true})
			if err != nil || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestAnimatedEditDoesNotSilentlyFlatten(t *testing.T) {
	pal := color.Palette{color.Transparent, color.White}
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	var source bytes.Buffer
	if err := gif.EncodeAll(&source, &gif.GIF{Image: []*image.Paletted{frame, frame}, Delay: []int{4, 9}, LoopCount: 3}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(cutoutPNG())}}})
	}))
	defer srv.Close()
	result, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).Image(context.Background(), "保留动画并换背景", source.Bytes())
	if err != nil || calls.Load() != 2 {
		t.Fatalf("animated edit must process every frame: calls=%d err=%v", calls.Load(), err)
	}
	animation, err := gif.DecodeAll(bytes.NewReader(result))
	if err != nil || len(animation.Image) != 2 || animation.Delay[0] != 4 || animation.Delay[1] != 9 || animation.LoopCount != 3 {
		t.Fatalf("animation lost timing/loop: %+v %v", animation, err)
	}
}

func cutoutPNG() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	img.SetNRGBA(1, 1, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func TestCutoutRejectsOpaqueAndEmptyResultsWithoutRetry(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "opaque", true: "empty"}[empty], func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
			if !empty {
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
					}
				}
			}
			var encoded bytes.Buffer
			_ = png.Encode(&encoded, img)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(encoded.Bytes())}}})
			}))
			defer srv.Close()
			result, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).Image(context.Background(), "抠图", cutoutPNG(), ImageOptions{TransparentBackground: true})
			if err == nil || len(result) != 0 || calls != 1 || classify(err).Detail != "transparency_missing" {
				t.Fatalf("invalid cutout accepted/retried: calls=%d err=%v", calls, err)
			}
		})
	}
}

func animationFixture(t *testing.T, frames int) []byte {
	t.Helper()
	pal := color.Palette{color.Transparent, color.White}
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	frame.SetColorIndex(0, 0, 1)
	animation := &gif.GIF{LoopCount: 3}
	for range frames {
		animation.Image = append(animation.Image, frame)
		animation.Delay = append(animation.Delay, 4)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, animation); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestGIFEditLimitRejectsBeforeModelCalls(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer srv.Close()
	_, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).Image(context.Background(), "改成水彩", animationFixture(t, MaxAIEditFrames+1))
	if err == nil || calls.Load() != 0 || classify(err).Detail != "gif_edit_limits" {
		t.Fatalf("oversized animation reached provider: %v calls=%d", err, calls.Load())
	}
}

func TestGIFEditFailureCancelsOutstandingFramesWithoutRetry(t *testing.T) {
	var calls, canceled atomic.Int32
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		if n == 3 {
			close(ready)
		}
		select {
		case <-ready:
		case <-time.After(2 * time.Second):
			t.Error("frame workers did not start")
			return
		}
		if n == 1 {
			w.WriteHeader(400)
			return
		}
		select {
		case <-r.Context().Done():
			canceled.Add(1)
		case <-time.After(2 * time.Second):
			t.Error("other frames were not canceled")
		}
	}))
	defer srv.Close()
	result, err := New(Settings{Model: "drawing", BaseURL: srv.URL}).Image(context.Background(), "换背景", animationFixture(t, 6))
	srv.Close()
	if err == nil || len(result) != 0 || calls.Load() != 3 || canceled.Load() != 2 {
		t.Fatalf("partial output/retry or missing cancellation: calls=%d canceled=%d err=%v", calls.Load(), canceled.Load(), err)
	}
}
