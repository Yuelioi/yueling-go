package system

import (
	"sync"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func TestQuotationHelpFollowsArgumentSpaceConfig(t *testing.T) {
	old := config.C.Bot.CommandArgSpaceRequired
	t.Cleanup(func() { config.C.Bot.CommandArgSpaceRequired = old })

	config.C.Bot.CommandArgSpaceRequired = false
	if call, add := quotationHelpSyntax(); call != "语录[名字]" || add != "添加语录[昵称]" {
		t.Fatalf("optional-space help = %q / %q", call, add)
	}

	config.C.Bot.CommandArgSpaceRequired = true
	if call, add := quotationHelpSyntax(); call != "语录 <名字>" || add != "添加语录 <昵称>" {
		t.Fatalf("required-space help = %q / %q", call, add)
	}
}

func resetCatalogTestRegistry() {
	finalizeOnce = sync.Once{}
	pluginByID = map[int]*pluginEntry{}
	pluginByName = map[string]*pluginEntry{}
	pluginByCmd = map[string]*pluginEntry{}
	pluginGroups = map[string][]*pluginEntry{}
	for i := range pluginRegistry {
		switch pluginRegistry[i].ID {
		case 18, 32:
			pluginRegistry[i].Usage = ""
			pluginRegistry[i].Commands = nil
		}
	}
}

func TestCatalogExportsStablePluginEntries(t *testing.T) {
	entries := Catalog()
	if len(entries) == 0 {
		t.Fatal("Catalog() returned no entries")
	}
	seen := map[int]bool{}
	var foundAI bool
	for _, entry := range entries {
		if entry.ID == 0 || entry.Name == "" || entry.Group == "" {
			t.Fatalf("invalid catalog entry: %+v", entry)
		}
		if seen[entry.ID] {
			t.Fatalf("duplicate plugin id %d", entry.ID)
		}
		seen[entry.ID] = true
		if entry.ID == 29 && entry.Name == "AI 助手" {
			foundAI = true
		}
	}
	if !foundAI {
		t.Fatalf("catalog missing AI 助手 entry")
	}
}

func TestCatalogReturnsCopiedCommands(t *testing.T) {
	entries := Catalog()
	if len(entries) == 0 {
		t.Fatal("Catalog() returned no entries")
	}
	entries[0].Commands[0] = "mutated"

	next := Catalog()
	if next[0].Commands[0] == "mutated" {
		t.Fatal("Catalog() returned commands aliased to shared state")
	}
}

func TestCatalogDoesNotConsumeHelpRegistryFinalization(t *testing.T) {
	resetCatalogTestRegistry()

	_ = Catalog()
	if len(pluginByID) != 0 || len(pluginByName) != 0 || len(pluginByCmd) != 0 || len(pluginGroups) != 0 {
		t.Fatal("Catalog() mutated help registry indexes")
	}

	RegisterHelp(bot.New())
	if pluginByID[30] == nil {
		t.Fatal("RegisterHelp() did not finalize pluginByID after Catalog()")
	}
	if pluginByName["帮助"] == nil {
		t.Fatal("RegisterHelp() did not finalize pluginByName after Catalog()")
	}
}

func TestCatalogUsesStablePluginIDConstants(t *testing.T) {
	if catalog.PluginAIAssistant != 29 {
		t.Fatalf("catalog.PluginAIAssistant = %d, want 29", catalog.PluginAIAssistant)
	}

	for _, entry := range Catalog() {
		if entry.ID == catalog.PluginAIAssistant {
			if entry.Name != "AI 助手" {
				t.Fatalf("catalog entry %d name = %q, want AI 助手", entry.ID, entry.Name)
			}
			return
		}
	}
	t.Fatalf("catalog missing plugin id %d", catalog.PluginAIAssistant)
}
