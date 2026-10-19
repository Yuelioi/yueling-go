package imageassets

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRelocatePreservesContentAndOwnership(t *testing.T) {
	for _, mode := range []string{"move", "rename", "random", "missing_name", "conflict", "recycle_failure", "stale", "cancel", "external"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			l := NewLibrary(root, []Category{{Name: "语录", Upload: true, NameRequired: true, Naming: "quotation"}, {Name: "表情", Aliases: []string{"表情包"}, Upload: true, NameRequired: true, Naming: "optional_label"}, {Name: "随机", Upload: true, Naming: "hash"}, {Name: "外站", External: true}})
			category := "语录"
			if mode == "random" {
				category = "随机"
			}
			for _, dir := range []string{category, "表情"} {
				if err := os.MkdirAll(filepath.Join(root, "images", dir), 0755); err != nil {
					t.Fatal(err)
				}
			}
			const original = "123_旧名_abc.png"
			writeAsset(t, root, category, original, "original bytes")
			rows, err := l.List(context.Background(), category, "", 1)
			if err != nil {
				t.Fatal(err)
			}
			id := rows.Assets[0].ID
			name, destination := "开心", "表情包"
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte("original bytes")))[:16]
			if mode == "missing_name" {
				name = ""
			}
			if mode == "external" {
				destination = "外站"
			}
			if mode == "conflict" {
				writeAsset(t, root, "表情", "开心_"+hash+".png", "other bytes")
			}
			if mode == "recycle_failure" {
				if err := os.WriteFile(filepath.Join(root, "trash"), []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "stale" {
				writeAsset(t, root, category, original, "changed bytes and size")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			var result MoveResult
			if mode == "rename" || mode == "random" {
				result, err = l.Rename(ctx, category, id, name)
				destination = category
			} else {
				result, err = l.Move(ctx, category, id, destination, name, 999)
			}
			success := mode == "move" || mode == "rename" || mode == "random"
			if !success {
				if err == nil {
					t.Fatal("expected rejection")
				}
				if _, e := os.Stat(filepath.Join(root, "images", category, original)); e != nil {
					t.Fatal("source lost", e)
				}
				if result.Copied != (mode == "recycle_failure") {
					t.Fatalf("unexpected result %+v", result)
				}
				if mode == "recycle_failure" {
					data, e := os.ReadFile(filepath.Join(root, "images", "表情", result.Destination))
					if e != nil || string(data) != "original bytes" {
						t.Fatal("destination lost", e)
					}
				}
				return
			}
			if err != nil || !result.SourceRemoved {
				t.Fatalf("%+v %v", result, err)
			}
			if mode == "rename" && !strings.HasPrefix(result.Destination, "123_开心_") {
				t.Fatal("quotation ownership changed", result)
			}
			if mode == "random" && !strings.HasPrefix(result.Destination, "开心_") {
				t.Fatal("name ignored", result)
			}
			if destination == "表情包" {
				destination = "表情"
			}
			data, e := os.ReadFile(filepath.Join(root, "images", destination, result.Destination))
			if e != nil || string(data) != "original bytes" {
				t.Fatal("content lost", e)
			}
			trash, _ := filepath.Glob(filepath.Join(root, "trash", "images", category, "*", original))
			wantTrash := 1
			if mode == "rename" || mode == "random" {
				wantTrash = 0
			}
			if len(trash) != wantTrash {
				t.Fatal("unexpected recycled files", trash)
			}
			if _, e := os.Stat(filepath.Join(root, "images", category, original)); !os.IsNotExist(e) {
				t.Fatal("original remains active", e)
			}
		})
	}
}

func TestRenamePreservesFileIdentityAndContentHash(t *testing.T) {
	root := t.TempDir()
	l := NewLibrary(root, []Category{{Name: "表情", Upload: true, NameRequired: true, Naming: "optional_label"}})
	dir := filepath.Join(root, "images", "表情")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"0123456789abcdef", "0123456789abcdef0123456789abcdef"} {
		file := "旧名字_" + hash + ".png"
		writeAsset(t, root, "表情", file, "image bytes")
		before, err := os.Stat(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		page, err := l.List(context.Background(), "表情", "旧名字_"+hash+".", 1)
		if err != nil || len(page.Assets) != 1 {
			t.Fatalf("%+v %v", page, err)
		}
		result, err := l.Rename(context.Background(), "表情", page.Assets[0].ID, "新名字")
		if err != nil || result.Copied || !result.SourceRemoved || result.Destination != "新名字_"+hash+".png" {
			t.Fatalf("rename changed the hash or copied the file: %+v %v", result, err)
		}
		after, err := os.Stat(filepath.Join(dir, result.Destination))
		if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
			t.Fatalf("rename recreated the file: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "trash")); !os.IsNotExist(err) {
		t.Fatalf("rename created trash: %v", err)
	}
}

func TestRenameRejectsConflictsAndStaleTargets(t *testing.T) {
	for _, mode := range []string{"conflict", "symlink", "canceled", "stale", "unchanged", "blocked_trash"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			l := NewLibrary(root, []Category{{Name: "表情", Upload: true, Naming: "optional_label"}})
			dir := filepath.Join(root, "images", "表情")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			const hash = "0123456789abcdef"
			const source = "旧名_" + hash + ".png"
			const target = "新名_" + hash + ".png"
			writeAsset(t, root, "表情", source, "original")
			page, err := l.List(context.Background(), "表情", "", 1)
			if err != nil {
				t.Fatal(err)
			}
			name := "新名"
			wantErr := ErrConflict
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "conflict":
				writeAsset(t, root, "表情", target, "keep existing target")
			case "symlink":
				if err := os.Symlink(source, filepath.Join(dir, target)); err != nil {
					t.Fatal(err)
				}
			case "canceled":
				cancel()
				wantErr = context.Canceled
			case "stale":
				writeAsset(t, root, "表情", source, "changed content")
				wantErr = ErrAsset
			case "unchanged":
				name = "旧名"
				wantErr = ErrUnchanged
			case "blocked_trash":
				if err := os.WriteFile(filepath.Join(root, "trash"), []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := l.Rename(ctx, "表情", page.Assets[0].ID, name)
			if mode == "blocked_trash" {
				if err != nil || !result.SourceRemoved || result.Copied {
					t.Fatalf("rename touched trash: %+v %v", result, err)
				}
				return
			}
			if !errors.Is(err, wantErr) || result.SourceRemoved || result.Copied {
				t.Fatalf("%+v %v", result, err)
			}
			if _, err := os.Stat(filepath.Join(dir, source)); err != nil {
				t.Fatal("source lost", err)
			}
			if mode == "conflict" {
				data, err := os.ReadFile(filepath.Join(dir, target))
				if err != nil || string(data) != "keep existing target" {
					t.Fatal("existing target overwritten", err)
				}
			}
		})
	}
}

func TestConcurrentRenameDoesNotReplaceExistingTarget(t *testing.T) {
	root := t.TempDir()
	l := NewLibrary(root, []Category{{Name: "表情", Upload: true, Naming: "optional_label"}})
	dir := filepath.Join(root, "images", "表情")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		writeAsset(t, root, "表情", name+"_0123456789abcdef.png", "same image bytes")
	}
	page, err := l.List(context.Background(), "表情", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, asset := range page.Assets {
		wg.Go(func() { _, err := l.Rename(context.Background(), "表情", asset.ID, "新名"); results <- err })
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	page, err = l.List(context.Background(), "表情", "", 1)
	if err != nil || page.Total != 2 {
		t.Fatalf("source or target lost: %+v %v", page, err)
	}
}
