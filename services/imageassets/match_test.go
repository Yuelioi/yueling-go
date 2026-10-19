package imageassets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yuelioi/yueling-go/services"
)

func TestMatchImageReturnsAllExactCandidates(t *testing.T) {
	l, root := libraryFixture(t)
	data := fixture(t)
	for i := range PageSize + 2 {
		writeAsset(t, root, "本地图", fmt.Sprintf("旧名字%02d.png", i), string(data))
	}
	writeAsset(t, root, "远程图", "外站.png", string(data))
	writeAsset(t, root, "本地图", "unrelated.png", "different")
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "images", "本地图", "link.png")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		category, query    string
		page, total, count int
	}{{"", "", 1, 22, 20}, {"发图", "", 2, 22, 2}, {"", "旧名字00", 1, 1, 1}, {"", "不存在", 1, 0, 0}} {
		page, err := l.MatchImage(context.Background(), data, tc.category, tc.query, tc.page, Options{})
		if err != nil || page.Total != tc.total || len(page.Assets) != tc.count {
			t.Fatalf("%+v: %+v %v", tc, page, err)
		}
		for _, item := range page.Assets {
			f, err := l.Open(context.Background(), item.Category, item.ID)
			if err != nil || item.Category != "本地图" {
				t.Fatalf("match did not return a real local ID: %+v %v", item, err)
			}
			f.Close()
		}
	}
	for _, tc := range []struct {
		category string
		page     int
	}{{"远程图", 1}, {"../本地图", 1}, {"", 0}} {
		if _, err := l.MatchImage(context.Background(), data, tc.category, "", tc.page, Options{}); err == nil {
			t.Fatal("accepted invalid match scope", tc)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.MatchImage(ctx, data, "", "", 1, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := l.MatchImage(context.Background(), []byte("invalid"), "", "", 1, Options{}); err == nil {
		t.Fatal("invalid image accepted")
	}
}

func TestMatchImageHandlesUploadAndSendConversion(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 768, 768))
	var seed uint32 = 42
	for i := range img.Pix {
		seed = seed*1664525 + 1013904223
		img.Pix[i] = byte(seed >> 24)
		if i%4 == 3 {
			img.Pix[i] = 255
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	compressed := services.ShrinkToJPEG(data, 1<<20, 85)
	if len(data) <= 1<<20 || bytes.Equal(data, compressed) {
		t.Fatal("fixture did not exercise compression")
	}
	for _, mode := range []string{"uploaded original", "reply to sent image"} {
		t.Run(mode, func(t *testing.T) {
			l, root := libraryFixture(t)
			stored, quoted := compressed, data
			if mode == "reply to sent image" {
				stored, quoted = data, compressed
			}
			writeAsset(t, root, "本地图", "legacy-name.png", string(stored))
			page, err := l.MatchImage(context.Background(), quoted, "", "", 1, Options{Convert: true, MinBytes: 1 << 20, Quality: 85})
			if err != nil || page.Total != 1 {
				t.Fatalf("%+v %v", page, err)
			}
		})
	}
}

func TestMatchImageKeepsDuplicatesInDifferentCategories(t *testing.T) {
	l, root := libraryFixture(t)
	data := fixture(t)
	l.categories = append(l.categories, Category{Name: "第二类目", Aliases: []string{"别名"}})
	if err := os.MkdirAll(filepath.Join(root, "images", "第二类目"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, category := range []string{"本地图", "第二类目"} {
		writeAsset(t, root, category, "同图.png", string(data))
	}
	page, err := l.MatchImage(context.Background(), data, "", "", 1, Options{})
	if err != nil || page.Total != 2 || page.Assets[0].Category == page.Assets[1].Category || page.Assets[0].ID == page.Assets[1].ID {
		t.Fatalf("distinct copies were collapsed: %+v %v", page, err)
	}
	filtered, err := l.MatchImage(context.Background(), data, "别名", "", 1, Options{})
	if err != nil || filtered.Total != 1 || filtered.Assets[0].Category != "第二类目" {
		t.Fatalf("category did not narrow candidates: %+v %v", filtered, err)
	}
}
