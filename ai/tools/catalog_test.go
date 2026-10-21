package tools

import (
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/ai"
	openai "github.com/sashabaranov/go-openai"
)

func TestProductionCatalogContainsOnlySelectedCapabilities(t *testing.T) {
	registry := ai.NewRegistry()
	RegisterAll(registry, Dependencies{})
	want := []string{"edit_image", "fetch_webpage", "generate_image", "get_chat_history", "get_group_members", "get_user_avatar", "inspect_image", "resolve_user_by_name", "search_group_knowledge", "web_search"}
	var names []string
	var schemas []openai.Tool
	for _, tool := range registry.AllTools() {
		names = append(names, tool.Name)
		schemas = append(schemas, tool.Schema())
		for _, dependency := range tool.Requires {
			if !slices.Contains(want, dependency) {
				t.Errorf("%s still requires removed capability %s", tool.Name, dependency)
			}
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("production capabilities = %v, want %v", names, want)
	}
	data, err := json.Marshal(schemas)
	if err != nil {
		t.Fatal(err)
	}
	if chars := utf8.RuneCount(data); chars > 4500 {
		t.Fatalf("selected native schemas grew to %d characters", chars)
	}
}
