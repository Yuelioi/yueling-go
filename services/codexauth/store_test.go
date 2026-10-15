package codexauth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = `{"auth_mode":"chatgpt","tokens":{"access_token":"access-private","refresh_token":"refresh-private","account_id":"test-account","id_token":"identity-private"},"other_secret":"drop"}`

func TestCredentialImportPrivateAndExplicitReplace(t *testing.T) {
	dir := t.TempDir()
	if err := Import(dir, "account", []byte(fixture), false); err != nil {
		t.Fatal(err)
	}
	p, _ := Directory(dir, "account")
	info, err := os.Stat(filepath.Join(p, "auth.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal(info, err)
	}
	body, _ := os.ReadFile(filepath.Join(p, "auth.json"))
	if strings.Contains(string(body), "other_secret") {
		t.Fatal("unexpected payload copied")
	}
	if err := Import(dir, "account", []byte(fixture), false); err == nil {
		t.Fatal("overwrote existing token")
	}
	if err := Import(dir, "account", []byte(fixture), true); err != nil {
		t.Fatal(err)
	}
	state := ReadStatus(dir, "account")
	if !state.Imported || state.AccountID != "test-account" {
		t.Fatal(state)
	}
	if err := Import(dir, "../escape", []byte(fixture), true); err == nil {
		t.Fatal("path traversal")
	}
	for _, data := range []string{`{"OPENAI_API_KEY":"secret"}`, `{"tokens":{"access_token":"secret"}}`, strings.Repeat("x", 65537)} {
		if _, err := Normalize([]byte(data)); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	}
}
