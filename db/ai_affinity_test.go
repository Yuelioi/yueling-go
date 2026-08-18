package db

import (
	"path/filepath"
	"sync"
	"testing"
)

func initTempAIAffinityDB(t *testing.T) {
	t.Helper()
	if err := initSQLiteForTest(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestUpdateAIAffinityPersistsScoreReasonAndNickname(t *testing.T) {
	initTempAIAffinityDB(t)

	row, err := UpdateAIAffinity(1, 100, "alice", 50, 2, 0, 100, "normal_chat")
	if err != nil {
		t.Fatalf("update affinity: %v", err)
	}
	if row.Score != 52 {
		t.Fatalf("score = %d, want 52", row.Score)
	}
	if row.LastReason != "normal_chat" {
		t.Fatalf("last reason = %q, want normal_chat", row.LastReason)
	}
	if row.Nickname != "alice" {
		t.Fatalf("nickname = %q, want alice", row.Nickname)
	}

	got, err := GetAIAffinity(1, 100)
	if err != nil {
		t.Fatalf("get affinity: %v", err)
	}
	if got.Score != 52 {
		t.Fatalf("persisted score = %d, want 52", got.Score)
	}
}

func TestReplaceAIAffinityTiersPersistsOrderedSettingsAndReset(t *testing.T) {
	initTempAIAffinityDB(t)

	tiers := []AIAffinityTier{
		{MinScore: 70, Name: "亲近", Prompt: "可以使用更熟悉的语气。"},
		{MinScore: 0, Name: "普通", Prompt: "保持礼貌自然。"},
	}
	if err := ReplaceAIAffinityTiers(tiers); err != nil {
		t.Fatalf("ReplaceAIAffinityTiers() error = %v", err)
	}

	got, err := ListAIAffinityTiers()
	if err != nil {
		t.Fatalf("ListAIAffinityTiers() error = %v", err)
	}
	if len(got) != 2 || got[0].MinScore != 0 || got[0].Name != "普通" || got[1].MinScore != 70 {
		t.Fatalf("stored tiers = %+v", got)
	}

	if err := ReplaceAIAffinityTiers(nil); err != nil {
		t.Fatalf("reset tiers: %v", err)
	}
	got, err = ListAIAffinityTiers()
	if err != nil || len(got) != 0 {
		t.Fatalf("tiers after reset = %+v, err = %v", got, err)
	}
}

func TestUpdateAIAffinityClampsScoreToMinimum(t *testing.T) {
	initTempAIAffinityDB(t)

	row, err := UpdateAIAffinity(1, 100, "", 50, -99, 0, 100, "harmful_content")
	if err != nil {
		t.Fatalf("update affinity: %v", err)
	}
	if row.Score != 0 {
		t.Fatalf("score = %d, want 0", row.Score)
	}
	if row.LastReason != "harmful_content" {
		t.Fatalf("last reason = %q, want harmful_content", row.LastReason)
	}
}

func TestUpdateAIAffinityKeepsLastReasonForNeutralMessage(t *testing.T) {
	initTempAIAffinityDB(t)

	row, err := UpdateAIAffinity(10, 20, "tester", 50, 2, 0, 100, "polite")
	if err != nil {
		t.Fatal(err)
	}
	row, err = UpdateAIAffinity(10, 20, "tester", 50, 0, 0, 100, "")
	if err != nil {
		t.Fatal(err)
	}
	if row.Score != 52 || row.LastReason != "polite" {
		t.Fatalf("neutral update = %+v, want score=52 reason=polite", row)
	}
}

func TestUpdateAIAffinityAppliesConcurrentDeltas(t *testing.T) {
	initTempAIAffinityDB(t)

	if _, err := UpdateAIAffinity(1, 100, "alice", 50, 0, 0, 100, "normal_chat"); err != nil {
		t.Fatalf("initialize affinity: %v", err)
	}

	const updates = 20
	var wg sync.WaitGroup
	errs := make(chan error, updates)
	for range updates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := UpdateAIAffinity(1, 100, "alice", 50, 1, 0, 100, "normal_chat")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("update affinity: %v", err)
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
