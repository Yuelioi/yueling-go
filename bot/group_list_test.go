package bot

import (
	"encoding/json"
	"testing"
)

func TestParseGroupList(t *testing.T) {
	raw := json.RawMessage(`[{"group_id":100,"group_name":"A"},{"group_id":200,"group_name":"B"}]`)
	groups, err := parseGroupList(raw)
	if err != nil {
		t.Fatalf("parseGroupList() error = %v", err)
	}
	if len(groups) != 2 || groups[0].GroupID != 100 || groups[0].GroupName != "A" || groups[1].GroupID != 200 {
		t.Fatalf("groups = %+v", groups)
	}
}
