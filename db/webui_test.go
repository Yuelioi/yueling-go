package db

import "testing"

func TestGroupPluginDisabledCRUDAndBatch(t *testing.T) {
	initTempAIAffinityDB(t)

	disabled, err := IsGroupPluginDisabled(100, 29)
	if err != nil {
		t.Fatalf("initial disabled: %v", err)
	}
	if disabled {
		t.Fatalf("disabled = true, want false")
	}

	if err := SetGroupPluginDisabled(100, 29, true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	disabled, err = IsGroupPluginDisabled(100, 29)
	if err != nil || !disabled {
		t.Fatalf("disabled=%v err=%v, want true nil", disabled, err)
	}

	if err := SetGroupPluginDisabled(100, 29, false); err != nil {
		t.Fatalf("enable: %v", err)
	}
	disabled, err = IsGroupPluginDisabled(100, 29)
	if err != nil || disabled {
		t.Fatalf("disabled=%v err=%v, want false nil", disabled, err)
	}

	if err := SetPluginDisabledForGroups(34, []int64{100, 200}, true); err != nil {
		t.Fatalf("batch disable: %v", err)
	}
	for _, groupID := range []int64{100, 200} {
		disabled, err := IsGroupPluginDisabled(groupID, 34)
		if err != nil || !disabled {
			t.Fatalf("group %d disabled=%v err=%v, want true nil", groupID, disabled, err)
		}
	}
}

func TestAIAffinityAdminListAndMutations(t *testing.T) {
	initTempAIAffinityDB(t)

	if _, err := UpdateAIAffinity(1, 100, "alice", 50, 5, 0, 100, "normal"); err != nil {
		t.Fatalf("seed alice: %v", err)
	}
	if _, err := UpdateAIAffinity(2, 100, "bob", 50, -20, 0, 100, "bad"); err != nil {
		t.Fatalf("seed bob: %v", err)
	}
	if _, err := UpdateAIAffinity(3, 200, "carol", 50, 1, 0, 100, "normal"); err != nil {
		t.Fatalf("seed carol: %v", err)
	}

	rows, err := ListAIAffinityAdmin(100, "ali", 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Nickname != "alice" {
		t.Fatalf("rows = %+v, want alice only", rows)
	}

	row, err := SetAIAffinityScore(rows[0].ID, 999, 0, 100, "webui_set")
	if err != nil {
		t.Fatalf("set score: %v", err)
	}
	if row.Score != 100 || row.LastReason != "webui_set" {
		t.Fatalf("after set = %+v, want score 100 reason webui_set", row)
	}

	row, err = AdjustAIAffinityScore(row.ID, -150, 0, 100, "webui_adjust")
	if err != nil {
		t.Fatalf("adjust score: %v", err)
	}
	if row.Score != 0 || row.LastReason != "webui_adjust" {
		t.Fatalf("after adjust = %+v, want score 0 reason webui_adjust", row)
	}

	row, err = ResetAIAffinityScore(row.ID, 50, 0, 100, "webui_reset")
	if err != nil {
		t.Fatalf("reset score: %v", err)
	}
	if row.Score != 50 || row.LastReason != "webui_reset" {
		t.Fatalf("after reset = %+v, want score 50 reason webui_reset", row)
	}
}
