package tools

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
)

func TestImageEditReferenceArgumentsRejectBeforeDownloads(t *testing.T) {
	old := config.Current()
	t.Cleanup(func() { config.Publish(old) })
	cfg := old
	cfg.AI = config.AIConfig{Connections: []config.AIConnection{{ID: "fixture", Kind: "openai", BaseURL: "https://provider.example"}}, Profiles: []config.AIProfile{{ID: "image", ConnectionID: "fixture", Model: "drawing", Capabilities: []string{"image_edit"}}}, Bindings: map[string]string{"image_edit": "image"}}
	config.Publish(cfg)
	registry := ai.NewRegistry()
	(&toolset{registry: registry}).registerImageGeneration()
	edit, _ := registry.GetTool("edit_image")
	for _, args := range []string{
		`{"prompt":"新图"}`,
		`{"prompt":"新图","image_ids":[]}`,
		`{"prompt":"新图","image_ids":["a","b","c","d"]}`,
		`{"prompt":"新图","image_ids":["a","a"]}`,
		`{"prompt":"新图","image_ids":["a",""]}`,
		`{"prompt":"新图","image_id":"a","image_ids":["b"]}`,
	} {
		var params map[string]any
		_ = json.Unmarshal([]byte(args), &params)
		result := edit.ResultHandler(&ai.ToolContext{Params: params})
		if result.Status != ai.ToolRejected || result.PossibleSideEffects {
			t.Errorf("invalid batch wasn't rejected before session/network access: %s %+v", args, result)
		}
	}
}

func TestImageEditNormalizesLegacyReferencesWithoutLosingOrder(t *testing.T) {
	var legacy, single, ordered, reversed map[string]any
	_ = json.Unmarshal([]byte(`{"image_id":"image_1","prompt":" 新图 "}`), &legacy)
	_ = json.Unmarshal([]byte(`{"image_ids":["image_1"],"prompt":"新图","transparent_background":false}`), &single)
	_ = json.Unmarshal([]byte(`{"image_ids":["image_3","image_1","image_2"],"prompt":"新图"}`), &ordered)
	_ = json.Unmarshal([]byte(`{"image_ids":["image_1","image_3","image_2"],"prompt":"新图"}`), &reversed)
	if imageEditActionKey(legacy) == "" || imageEditActionKey(legacy) != imageEditActionKey(single) || imageEditActionKey(ordered) == imageEditActionKey(reversed) {
		t.Fatal("legacy alias can bypass deduplication, or reference order was discarded")
	}
	ids, err := imageEditReferences(ordered)
	if err != nil || !reflect.DeepEqual(ids, []string{"image_3", "image_1", "image_2"}) {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
}

func TestImageEditRejectsAnyUnknownReferenceBeforeProviderCall(t *testing.T) {
	calls := 0
	result, sent := dispatchImageFixture(t, []byte("not fetched"), "edit_image", `{"image_ids":["image_1","unknown"],"prompt":"融合参考图"}`, func(w http.ResponseWriter, r *http.Request) {
		calls++
	})
	if result.Status != ai.ToolRejected || result.PossibleSideEffects || !strings.Contains(result.Content, "参考图不可用") || calls != 0 || len(sent) != 0 {
		t.Fatalf("invalid reference partially executed: %+v calls=%d sent=%d", result, calls, len(sent))
	}
}
