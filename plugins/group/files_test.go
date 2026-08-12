package group

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func TestFilesFileKeyCannotEscapeBackupDirectory(t *testing.T) {
	key := filesFileKey(bot.QQFile{FolderName: "../../outside", FileName: "../secret.txt"})
	if filepath.IsAbs(key) || key == ".." || strings.HasPrefix(key, ".."+string(filepath.Separator)) {
		t.Fatalf("unsafe file key = %q", key)
	}
	base := t.TempDir()
	target, err := filepath.Abs(filepath.Join(base, key))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(target, base+string(filepath.Separator)) {
		t.Fatalf("file key escaped base: base=%q target=%q", base, target)
	}
}

func TestFilesSafeComponentIsStableAndCollisionResistant(t *testing.T) {
	first := filesSafeComponent("../same.txt")
	if first != filesSafeComponent("../same.txt") {
		t.Fatal("safe component is not stable")
	}
	if first == filesSafeComponent("..\\same.txt") {
		t.Fatal("different unsafe names collapsed to the same component")
	}
	if got := filesSafeComponent("normal.txt"); got != "normal.txt" {
		t.Fatalf("normal component changed to %q", got)
	}
}
