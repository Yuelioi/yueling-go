package db

import (
	"sync"
	"testing"
)

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

	if err := SetPluginDisabledForGroups(34, []int64{100, 200}, false); err != nil {
		t.Fatalf("batch enable: %v", err)
	}
	for _, groupID := range []int64{100, 200} {
		disabled, err := IsGroupPluginDisabled(groupID, 34)
		if err != nil || disabled {
			t.Fatalf("group %d disabled=%v err=%v, want false nil", groupID, disabled, err)
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

func TestListAIAffinityAdminNumericQueryMatchesNickname(t *testing.T) {
	initTempAIAffinityDB(t)

	if _, err := UpdateAIAffinity(777, 100, "12345", 50, 0, 0, 100, "normal"); err != nil {
		t.Fatalf("seed numeric nickname: %v", err)
	}

	rows, err := ListAIAffinityAdmin(100, "12345", 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].UserID != 777 || rows[0].Nickname != "12345" {
		t.Fatalf("rows = %+v, want user 777 with numeric nickname", rows)
	}
}

func TestListAIAffinityAdminNumericQueryStaysWithinGroup(t *testing.T) {
	initTempAIAffinityDB(t)

	if _, err := UpdateAIAffinity(777, 100, "12345", 50, 0, 0, 100, "normal"); err != nil {
		t.Fatalf("seed target group: %v", err)
	}
	if _, err := UpdateAIAffinity(12345, 200, "other", 50, 0, 0, 100, "normal"); err != nil {
		t.Fatalf("seed other group: %v", err)
	}

	rows, err := ListAIAffinityAdmin(100, "12345", 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].GroupID != 100 || rows[0].UserID != 777 {
		t.Fatalf("rows = %+v, want only target group numeric nickname", rows)
	}
}

func TestAIAffinityAdminAdjustAppliesConcurrentDeltas(t *testing.T) {
	initTempAIAffinityDB(t)

	row, err := UpdateAIAffinity(1, 100, "alice", 50, 0, 0, 100, "normal")
	if err != nil {
		t.Fatalf("seed alice: %v", err)
	}

	const updates = 20
	var wg sync.WaitGroup
	errs := make(chan error, updates)
	for range updates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := AdjustAIAffinityScore(row.ID, 1, 0, 100, "webui_adjust")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("adjust affinity: %v", err)
		}
	}

	got, err := GetAIAffinity(1, 100)
	if err != nil {
		t.Fatalf("get affinity: %v", err)
	}
	if got.Score != 70 {
		t.Fatalf("score = %d, want 70", got.Score)
	}
}

func TestAIAffinityAdminMutationsReturnErrorForMissingID(t *testing.T) {
	initTempAIAffinityDB(t)

	if row, err := SetAIAffinityScore(999, 60, 0, 100, "webui_set"); err == nil {
		t.Fatalf("set missing row = %+v err=nil, want error", row)
	}
	if row, err := AdjustAIAffinityScore(999, 1, 0, 100, "webui_adjust"); err == nil {
		t.Fatalf("adjust missing row = %+v err=nil, want error", row)
	}
	if row, err := ResetAIAffinityScore(999, 50, 0, 100, "webui_reset"); err == nil {
		t.Fatalf("reset missing row = %+v err=nil, want error", row)
	}
}
