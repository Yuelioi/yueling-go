package tools

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
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

func TestDetectImageExt(t *testing.T) {
	cases := []struct {
		head []byte
		want string
	}{
		{[]byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0, 0, 0, 0, 0}, "png"},
		{[]byte{'G', 'I', 'F', '8', '9', 'a', 0, 0, 0, 0, 0, 0}, "gif"},
		{[]byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}, "webp"},
		{[]byte{0xFF, 0xD8, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "jpg"},
		{[]byte{1, 2}, "jpg"},
	}
	for _, c := range cases {
		if got := detectImageExt(c.head); got != c.want {
			t.Fatalf("head=%v got=%q want=%q", c.head, got, c.want)
		}
	}
}

func TestWriteZipBytes(t *testing.T) {
	items := []packItem{
		{name: "001.jpg", data: []byte("aaa")},
		{name: "002.png", data: []byte("bb")},
	}
	raw, err := writeZipBytes(items)
	if err != nil {
		t.Fatalf("writeZipBytes: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("want 2 files, got %d", len(zr.File))
	}
	if zr.File[0].Name != "001.jpg" || zr.File[1].Name != "002.png" {
		t.Fatalf("names = %q,%q", zr.File[0].Name, zr.File[1].Name)
	}
	rc, _ := zr.File[0].Open()
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "aaa" {
		t.Fatalf("file0 content = %q", got)
	}
}

var errPackTest = errors.New("fail")

func TestDownloadItems(t *testing.T) {
	data := map[string][]byte{
		"u1": {0x89, 'P', 'N', 'G', 0, 0, 0, 0, 0, 0, 0, 0}, // png
		"u2": {0xFF, 0xD8, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // jpg
		"u3": nil,                                            // 下载失败
	}
	get := func(u string) ([]byte, error) {
		if data[u] == nil {
			return nil, errPackTest
		}
		return data[u], nil
	}
	items, total := downloadItems([]string{"u1", "u2", "u3"}, get, 100, 100*1024*1024)
	if len(items) != 2 {
		t.Fatalf("want 2 ok items, got %d", len(items))
	}
	if items[0].name != "001.png" || items[1].name != "002.jpg" {
		t.Fatalf("names = %q,%q", items[0].name, items[1].name)
	}
	if total != 24 {
		t.Fatalf("total bytes = %d", total)
	}
}
