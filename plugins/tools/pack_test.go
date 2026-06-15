package tools

import (
	"encoding/json"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func img(u string) bot.Segment {
	return bot.Segment{Type: "image", Data: json.RawMessage(`{"url":"` + u + `"}`)}
}
func fwd(id string) bot.Segment {
	return bot.Segment{Type: "forward", Data: json.RawMessage(`{"id":"` + id + `"}`)}
}

func TestCollectImages(t *testing.T) {
	store := map[string][]bot.Message{
		"f1": {{img("http://a/inner1.jpg"), fwd("f2")}},
		"f2": {{img("http://a/inner2.jpg"), fwd("f1")}},
	}
	getForward := func(id string) ([]bot.Message, error) { return store[id], nil }

	root := bot.Message{img("http://a/top.jpg"), fwd("f1")}
	var out []string
	visited := map[string]bool{}
	collectImages(root, getForward, 0, 100, visited, &out)

	want := []string{"http://a/top.jpg", "http://a/inner1.jpg", "http://a/inner2.jpg"}
	if len(out) != len(want) {
		t.Fatalf("got %v want %v", out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d]=%q want %q (full=%v)", i, out[i], want[i], out)
		}
	}
}

func TestCollectImagesMaxImages(t *testing.T) {
	getForward := func(id string) ([]bot.Message, error) { return nil, nil }
	root := bot.Message{img("http://a/1.jpg"), img("http://a/2.jpg"), img("http://a/3.jpg")}
	var out []string
	collectImages(root, getForward, 0, 2, map[string]bool{}, &out)
	if len(out) != 2 {
		t.Fatalf("maxImages=2 应只收 2 张, got %d (%v)", len(out), out)
	}
}
