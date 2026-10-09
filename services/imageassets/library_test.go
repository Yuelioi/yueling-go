package imageassets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func libraryFixture(t *testing.T) (*Library, string) {
	t.Helper()
	root := t.TempDir()
	for _, category := range []string{"本地图", "远程图"} {
		if err := os.MkdirAll(filepath.Join(root, "images", category), 0755); err != nil {
			t.Fatal(err)
		}
	}
	library := NewLibrary(root, []Category{{Name: "本地图", Aliases: []string{"发图", "带 空格"}}, {Name: "远程图", External: true}})
	return library, root
}
func writeAsset(t *testing.T, root, category, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "images", category, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestLibraryDeletesOnlyExactLocalAsset(t *testing.T) {
	l, root := libraryFixture(t)
	writeAsset(t, root, "本地图", "a.png", "a")
	writeAsset(t, root, "本地图", "b.png", "b")
	writeAsset(t, root, "本地图", "notes.txt", "keep")
	page, err := l.List(context.Background(), "发图", "", 1)
	if err != nil || len(page.Assets) != 2 {
		t.Fatalf("%+v %v", page, err)
	}
	id := page.Assets[0].ID

	name, err := l.Delete(context.Background(), "发图", id)
	if err != nil || name != "a.png" {
		t.Fatalf("%s %v", name, err)
	}
	if _, err := os.Stat(filepath.Join(root, "images", "本地图", "a.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("asset not removed")
	}
	trash, err := filepath.Glob(filepath.Join(root, "trash", "images", "本地图", "*", "a.png"))
	if err != nil || len(trash) != 1 {
		t.Fatalf("%v %v", trash, err)
	}
	data, err := os.ReadFile(trash[0])
	if err != nil || string(data) != "a" {
		t.Fatal("trash content lost")
	}
	if _, err := l.Delete(context.Background(), "本地图", id); !errors.Is(err, ErrAsset) {
		t.Fatal("repeated delete succeeded", err)
	}
	for _, name := range []string{"b.png", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(root, "images", "本地图", name)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestLibraryRejectsExternalPathsAndChangedIDs(t *testing.T) {
	l, root := libraryFixture(t)
	writeAsset(t, root, "本地图", "a.png", "a")
	writeAsset(t, root, "远程图", "a.png", "a")
	page, _ := l.List(context.Background(), "本地图", "", 1)
	id := page.Assets[0].ID
	for _, category := range []string{"远程图", "../本地图", "https://example.com", "未配置"} {
		if _, err := l.Delete(context.Background(), category, id); err == nil {
			t.Fatal("accepted", category)
		}
	}
	for _, id := range []string{"a.png", "../a.png", "https://example.com/a.png", strings.Repeat("0", 32)} {
		if _, err := l.Delete(context.Background(), "本地图", id); !errors.Is(err, ErrAsset) {
			t.Fatal(err)
		}
	}
	writeAsset(t, root, "本地图", "a.png", "replacement changed size")
	if _, err := l.Delete(context.Background(), "本地图", id); !errors.Is(err, ErrAsset) {
		t.Fatal("stale ID deleted replacement")
	}
	if _, err := os.Stat(filepath.Join(root, "images", "远程图", "a.png")); err != nil {
		t.Fatal("external category local lookalike changed", err)
	}
}
func TestLibraryRejectsSymlinkAndCancellation(t *testing.T) {
	l, root := libraryFixture(t)
	outside := t.TempDir()
	victim := filepath.Join(outside, "secret.png")
	if err := os.WriteFile(victim, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(root, "images", "本地图", "link.png")); err != nil {
		t.Fatal(err)
	}
	page, err := l.List(context.Background(), "本地图", "", 1)
	if err != nil || page.Total != 0 {
		t.Fatalf("symlink listed %+v %v", page, err)
	}
	writeAsset(t, root, "本地图", "a.png", "a")
	page, _ = l.List(context.Background(), "本地图", "", 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.Delete(ctx, "本地图", page.Assets[0].ID); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "trash")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Delete(context.Background(), "本地图", page.Assets[0].ID); err == nil {
		t.Fatal("escaped trash root")
	}
	if _, err := os.Stat(filepath.Join(root, "images", "本地图", "a.png")); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(victim); string(data) != "keep" {
		t.Fatal("outside file changed")
	}
}
func TestLibraryCategorySymlinkAndAmbiguity(t *testing.T) {
	l, root := libraryFixture(t)
	if err := os.Remove(filepath.Join(root, "images", "本地图")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("远程图", filepath.Join(root, "images", "本地图")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.List(context.Background(), "本地图", "", 1); !errors.Is(err, ErrUnsafe) {
		t.Fatal(err)
	}
	l.categories = append(l.categories, Category{Name: "远程图", Aliases: []string{"发图"}, External: true})
	if _, err := l.List(context.Background(), "发图", "", 1); !errors.Is(err, ErrCategory) {
		t.Fatal(err)
	}
}
func TestLibraryPagination(t *testing.T) {
	l, root := libraryFixture(t)
	for i := range 23 {
		writeAsset(t, root, "本地图", string(rune('a'+i))+".png", "x")
	}
	page, err := l.List(context.Background(), "发图", "", 2)
	if err != nil || page.Total != 23 || len(page.Assets) != 3 {
		t.Fatalf("%+v %v", page, err)
	}
}
