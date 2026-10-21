package bot

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestHistoryMessagePreservesDownloadableImageReferences(t *testing.T) {
	var message HistoryMessage
	if err := json.Unmarshal([]byte(`{"message_id":123,"message":[{"type":"text","data":{"text":"图片"}},{"type":"image","data":{"file":"thumbnail-file","url":"https://fixture.invalid/original.gif"}},{"type":"image","data":{"file":"https://fixture.invalid/image.png"}},{"type":"image","data":{}}]}`), &message); err != nil {
		t.Fatal(err)
	}
	want := []string{"https://fixture.invalid/original.gif", "https://fixture.invalid/image.png"}
	if got := message.ImageURLs(); !slices.Equal(got, want) {
		t.Fatalf("history image references=%v want=%v", got, want)
	}
}
