package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func baseConfig(extra string) string {
	return strings.TrimSpace(`
[bot]
name = "月灵"
data_dir = "data"

[napcat]
serve = ":9077"
token = ""

[ai]
deepseek_key = "sk-test"
`+"\n"+extra) + "\n"
}

func TestLoadWebUIDisabledAllowsEmptyPassword(t *testing.T) {
	path := writeConfig(t, baseConfig(`
[webui]
enabled = false
addr = ":9080"
password = ""
`))
	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if C.WebUI.Enabled {
		t.Fatalf("WebUI.Enabled = true, want false")
	}
}

func TestLoadWebUIEnabledRequiresPassword(t *testing.T) {
	path := writeConfig(t, baseConfig(`
[webui]
enabled = true
addr = ":9080"
password = ""
`))
	err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "webui.password is required") {
		t.Fatalf("Load() error = %v, want webui.password required", err)
	}
}

func TestLoadWebUIEnabledAcceptsPassword(t *testing.T) {
	path := writeConfig(t, baseConfig(`
[webui]
enabled = true
addr = ":9080"
password = "secret"
`))
	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !C.WebUI.Enabled || C.WebUI.Addr != ":9080" || C.WebUI.Password != "secret" {
		t.Fatalf("C.WebUI = %+v", C.WebUI)
	}
}

func TestLoadWebUIEnabledDefaultsAddr(t *testing.T) {
	path := writeConfig(t, baseConfig(`
[webui]
enabled = true
password = "secret"
`))
	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if C.WebUI.Addr != ":9080" {
		t.Fatalf("C.WebUI.Addr = %q, want :9080", C.WebUI.Addr)
	}
}

func TestLoadWebUIMissingSectionClearsPreviousValues(t *testing.T) {
	enabledPath := writeConfig(t, baseConfig(`
[webui]
enabled = true
addr = ":9080"
password = "secret"
`))
	if err := Load(enabledPath); err != nil {
		t.Fatalf("Load(enabled) error = %v", err)
	}

	missingPath := writeConfig(t, baseConfig(""))
	if err := Load(missingPath); err != nil {
		t.Fatalf("Load(missing webui) error = %v", err)
	}
	if C.WebUI.Enabled || C.WebUI.Password != "" {
		t.Fatalf("C.WebUI = %+v, want disabled with empty password", C.WebUI)
	}
}
