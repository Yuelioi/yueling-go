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
	"sync"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestSaveAtomicDedupNamesAndGroups(t *testing.T) {
	dir := t.TempDir()
	data := fixture(t)
	fetch := func(context.Context, string) ([]byte, error) { return data, nil }
	var wg sync.WaitGroup
	results := make(chan Result, 10)
	for range 10 {
		wg.Go(func() {
			results <- save(context.Background(), dir, []string{"image"}, func(h string) string { return "1_小明_" + h }, Options{}, fetch)
		})
	}
	wg.Wait()
	close(results)
	saved, dups := 0, 0
	for r := range results {
		saved += r.Saved
		dups += r.Duplicate
		if r.Failed != 0 {
			t.Fatal(r)
		}
	}
	if saved != 1 || dups != 9 {
		t.Fatalf("saved=%d duplicate=%d", saved, dups)
	}
	for _, name := range []string{"2_小明_", "1_小红_"} {
		r := save(context.Background(), dir, []string{"image"}, func(h string) string { return name + h }, Options{}, fetch)
		if r.Saved != 1 {
			t.Fatal(r)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 3 {
		t.Fatalf("%v %v", files, err)
	}
	for _, f := range files {
		b, e := os.ReadFile(filepath.Join(dir, f.Name()))
		if e != nil || !bytes.Equal(b, data) {
			t.Fatal("partial/corrupt image")
		}
	}
}
func TestSavePartialFailureAndCancellation(t *testing.T) {
	data := fixture(t)
	dir := t.TempDir()
	fetch := func(_ context.Context, u string) ([]byte, error) {
		if u == "bad" {
			return nil, errors.New("download failed")
		}
		return data, nil
	}
	r := save(context.Background(), dir, []string{"ok", "bad"}, func(h string) string { return h }, Options{}, fetch)
	if r.Saved != 1 || r.Failed != 1 {
		t.Fatal(r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r = save(ctx, dir, []string{"cancel"}, func(h string) string { return "cancel_" + h }, Options{}, func(context.Context, string) ([]byte, error) { cancel(); return data, nil })
	if r.Saved != 0 || r.Failed != 1 {
		t.Fatal(r)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal(files)
	}
}
func TestSaveRejectsInvalidImageAndName(t *testing.T) {
	valid := fixture(t)
	for _, tc := range []struct {
		name string
		data []byte
	}{{"../escape", valid}, {"bad", []byte("not an image")}, {"bad", valid[:30]}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			r := save(context.Background(), dir, []string{"image"}, func(string) string { return tc.name }, Options{}, func(context.Context, string) ([]byte, error) { return tc.data, nil })
			if r.Failed != 1 || r.Saved != 0 {
				t.Fatal(r)
			}
			files, _ := os.ReadDir(dir)
			if len(files) != 0 {
				t.Fatal(files)
			}
		})
	}
}
func TestFetchRejectsPrivateAndFileURLs(t *testing.T) {
	for _, url := range []string{"file:///etc/passwd", "http://127.0.0.1/image", "http://169.254.169.254/image"} {
		if _, err := Fetch(context.Background(), url); err == nil {
			t.Fatal(fmt.Sprintf("accepted %s", url))
		}
	}
}
