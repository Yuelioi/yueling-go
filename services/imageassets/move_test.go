package imageassets

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			if len(trash) != 1 {
				t.Fatal("missing recoverable original")
			}
			if _, e := os.Stat(filepath.Join(root, "images", category, original)); !os.IsNotExist(e) {
				t.Fatal("original remains active", e)
			}
		})
	}
}
