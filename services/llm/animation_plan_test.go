package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTextAnimationUsesFirstFrameAsIdentityReference(t *testing.T) {
	var calls atomic.Int32
	source := cutoutPNG()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			if r.URL.Path != "/images/generations" {
				t.Errorf("first frame path=%s", r.URL.Path)
			}
		} else {
			if r.URL.Path != "/images/edits" {
				t.Errorf("later frame path=%s", r.URL.Path)
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			f, _, err := r.FormFile("image")
			if err != nil {
				t.Error(err)
				return
			}
			data, _ := io.ReadAll(f)
			f.Close()
			if !bytes.Equal(data, source) {
				t.Error("later frame lost the generated identity reference")
			}
			if !strings.Contains(r.FormValue("prompt"), "固定角色长相") {
				t.Error("animation identity instruction missing")
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(source)}}})
	}))
	defer server.Close()
	data, err := New(Settings{Model: "drawing", BaseURL: server.URL}).Image(context.Background(), "白发角色", nil, ImageOptions{AnimationFrames: []string{"打哈欠", "揉眼睛", "闭眼睡觉"}, FrameDurationMS: 500})
	if err != nil {
		t.Fatal(err)
	}
	animation, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil || len(animation.Image) != 3 || calls.Load() != 3 || animation.LoopCount != 0 {
		t.Fatalf("calls=%d animation=%+v err=%v", calls.Load(), animation, err)
	}
	for _, delay := range animation.Delay {
		if delay != 50 {
			t.Fatalf("timing=%v", animation.Delay)
		}
	}
}

func TestInvalidAnimationPlanNeverCallsProvider(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	for _, opts := range []ImageOptions{
		{AnimationFrames: []string{"one"}},
		{AnimationFrames: make([]string, MaxAIEditFrames+1)},
		{AnimationFrames: []string{"one", " "}},
		{AnimationFrames: []string{"one", "two"}, FrameDurationMS: 1},
		{AnimationFrames: []string{"one", "two"}, FrameDurationMS: 5001},
		{FrameDurationMS: 500},
	} {
		data, err := New(Settings{Model: "drawing", BaseURL: server.URL}).Image(context.Background(), "动作GIF", nil, opts)
		if err == nil || len(data) != 0 || classify(err).Detail != "animation_plan" {
			t.Fatalf("invalid plan accepted: %+v err=%v", opts, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid animation made billable calls", calls.Load())
	}
}

func TestAnimationStopsAtQuotaWithoutRetryOrPartialGIF(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"b64_json": base64.StdEncoding.EncodeToString(cutoutPNG())}}})
	}))
	defer server.Close()
	data, err := New(Settings{Model: "drawing", BaseURL: server.URL}).Image(context.Background(), "动作GIF", nil, ImageOptions{AnimationFrames: []string{"一", "二", "三", "四"}})
	if err == nil || len(data) != 0 || calls.Load() != 3 {
		t.Fatalf("quota retried or partial GIF returned: calls=%d bytes=%d err=%v", calls.Load(), len(data), err)
	}
}

func TestExistingGIFCannotBeSilentlyReplacedByNewMotionPlan(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White})
	var source bytes.Buffer
	if err := gif.EncodeAll(&source, &gif.GIF{Image: []*image.Paletted{frame, frame}, Delay: []int{4, 9}}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	data, err := New(Settings{Model: "drawing", BaseURL: server.URL}).Image(context.Background(), "保留原GIF动作", source.Bytes(), ImageOptions{AnimationFrames: []string{"新动作一", "新动作二"}})
	if err == nil || classify(err).Detail != "animation_source_conflict" || len(data) != 0 || calls.Load() != 0 {
		t.Fatalf("existing timeline was silently discarded: calls=%d err=%v", calls.Load(), err)
	}
}
