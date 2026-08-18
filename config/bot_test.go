package config

import (
	"slices"
	"strings"
	"testing"
)

func TestLoadMigratesLegacyOwnerIDIntoSuperusers(t *testing.T) {
	content := strings.Replace(baseConfig(""), `data_dir = "data"`, "owner_id = 123\nsuperusers = [456]\ndata_dir = \"data\"", 1)
	path := writeConfig(t, content)
	if err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !slices.Contains(C.Bot.SuperUsers, int64(123)) || !slices.Contains(C.Bot.SuperUsers, int64(456)) {
		t.Fatalf("superusers = %v, want legacy owner and configured superuser", C.Bot.SuperUsers)
	}
}

func TestLoadCommandArgumentSpacePolicy(t *testing.T) {
	defaultPath := writeConfig(t, baseConfig(""))
	if err := Load(defaultPath); err != nil {
		t.Fatalf("Load(default) error = %v", err)
	}
	if !C.Bot.CommandArgSpaceRequired {
		t.Fatal("default bot.command_arg_space_required = false, want true")
	}

	optionalConfig := strings.Replace(baseConfig(""), `data_dir = "data"`, "data_dir = \"data\"\ncommand_arg_space_required = false", 1)
	optionalPath := writeConfig(t, optionalConfig)
	if err := Load(optionalPath); err != nil {
		t.Fatalf("Load(optional) error = %v", err)
	}
	if C.Bot.CommandArgSpaceRequired {
		t.Fatal("configured bot.command_arg_space_required = true, want false")
	}
}
