package system

import "testing"

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
