package imageassets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func uploadFixture(t *testing.T) (*Library, string, []byte) {
	t.Helper()
	root := t.TempDir()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return NewLibrary(root, []Category{
		{Name: "吃的", Upload: true, NameRequired: true, Naming: "label"},
		{Name: "语录", Upload: true, NameRequired: true, Naming: "quotation"},
		{Name: "表情", Aliases: []string{"表情包"}, Upload: true, NameRequired: true, Naming: "optional_label"},
		{Name: "随机", Upload: true, Naming: "hash"}, {Name: "只读"}, {Name: "外站", External: true},
	}), root, b.Bytes()
}
func TestUploadCategoryNamingAndDeduplication(t *testing.T) {
	l, root, data := uploadFixture(t)
	hash := fmt.Sprintf("%x", sha256.Sum256(data))[:16]
	for _, tc := range []struct {
		category, name, want string
		group                int64
	}{
		{"吃的", " 红烧肉🍖 ", "红烧肉_" + hash + ".png", 0},
		{"语录", "小明", "123_小明_" + hash + ".png", 123},
		{"表情包", "开心", "开心_" + hash + ".png", 0},
		{"随机", "忽略名称", hash + ".png", 0},
	} {
		got, err := l.Upload(context.Background(), tc.category, tc.name, tc.group, data, Options{Convert: true})
		if err != nil || got.Name != tc.want || got.Duplicate {
			t.Fatalf("%s: %+v %v", tc.category, got, err)
		}
		got, err = l.Upload(context.Background(), tc.category, tc.name, tc.group, data, Options{Convert: true})
		if err != nil || !got.Duplicate {
			t.Fatalf("duplicate: %+v %v", got, err)
		}
	}
	// Ownership and labels form part of the deduplication key.
	for _, tc := range []struct {
		name  string
		group int64
	}{{"小明", 456}, {"小红", 123}} {
		if got, err := l.Upload(context.Background(), "语录", tc.name, tc.group, data, Options{}); err != nil || got.Duplicate {
			t.Fatal(got, err)
		}
	}
	saved, err := os.ReadFile(filepath.Join(root, "images", "吃的", "红烧肉_"+hash+".png"))
	if err != nil || !bytes.Equal(saved, data) {
		t.Fatal("transparent PNG changed", err)
	}
}
func TestUploadRejectsInvalidInputsWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		category, name string
		group          int64
		badData        bool
	}{
		{"../逃逸", "a", 0, false}, {"外站", "a", 0, false}, {"只读", "a", 0, false},
		{"吃的", "", 0, false}, {"表情", "../a", 0, false}, {"语录", "a", 0, false}, {"语录", "a", -1, false},
		{"吃的", "正常", 0, true},
	} {
		t.Run(tc.category+tc.name+fmt.Sprint(tc.group, tc.badData), func(t *testing.T) {
			l, root, data := uploadFixture(t)
			if tc.badData {
				data = []byte("not an image")
			}
			if _, err := l.Upload(context.Background(), tc.category, tc.name, tc.group, data, Options{}); err == nil {
				t.Fatal("accepted invalid input")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 0 {
				t.Fatal("invalid request wrote files", entries)
			}
		})
	}
	l, root, data := uploadFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.Upload(ctx, "吃的", "a", 0, data, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("cancelled request wrote files")
	}
}
func TestUploadNeverFollowsLinksOrOverwritesConflicts(t *testing.T) {
	for _, target := range []string{"images", "category", "file", "conflict"} {
		t.Run(target, func(t *testing.T) {
			l, root, data := uploadFixture(t)
			dir := filepath.Join(root, "images", "吃的")
			hash := fmt.Sprintf("%x", sha256.Sum256(data))[:16]
			dest := filepath.Join(dir, "米饭_"+hash+".png")
			outside := t.TempDir()
			switch target {
			case "images":
				if err := os.Symlink(outside, filepath.Join(root, "images")); err != nil {
					t.Fatal(err)
				}
			case "category":
				os.MkdirAll(filepath.Join(root, "images"), 0755)
				if err := os.Symlink(outside, dir); err != nil {
					t.Fatal(err)
				}
			default:
				os.MkdirAll(dir, 0755)
				if target == "file" {
					os.Symlink(filepath.Join(outside, "keep.png"), dest)
				} else {
					os.WriteFile(dest, []byte("keep"), 0644)
				}
			}
			_, err := l.Upload(context.Background(), "吃的", "米饭", 0, data, Options{})
			if target == "conflict" {
				if !errors.Is(err, ErrConflict) {
					t.Fatal(err)
				}
				got, _ := os.ReadFile(dest)
				if string(got) != "keep" {
					t.Fatal("overwrote conflict")
				}
			} else if !errors.Is(err, ErrUnsafe) {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(outside)
			if len(entries) != 0 {
				t.Fatal("escaped root")
			}
			temps, _ := filepath.Glob(filepath.Join(dir, ".upload-*"))
			if len(temps) != 0 {
				t.Fatal("leaked temporary files")
			}
		})
	}
}
func TestUploadConcurrentRequestsAndAnimatedGIF(t *testing.T) {
	l, root, _ := uploadFixture(t)
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Transparent, color.White})
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, &gif.GIF{Image: []*image.Paletted{frame, frame}, Delay: []int{10, 20}, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	var wg sync.WaitGroup
	results := make(chan UploadResult, 8)
	failures := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			got, err := l.Upload(context.Background(), "表情", "转圈", 0, data, Options{Convert: true})
			results <- got
			failures <- err
		})
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	saved := 0
	name := ""
	for got := range results {
		if !got.Duplicate {
			saved++
		}
		name = got.Name
	}
	if saved != 1 {
		t.Fatal("non-idempotent upload", saved)
	}
	got, err := os.ReadFile(filepath.Join(root, "images", "表情", name))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("GIF bytes changed", err)
	}
	decoded, err := gif.DecodeAll(bytes.NewReader(got))
	if err != nil || len(decoded.Image) != 2 || decoded.Delay[1] != 20 {
		t.Fatal("animation lost", err)
	}
}
