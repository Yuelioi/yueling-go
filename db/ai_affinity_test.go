package db

import (
	"path/filepath"
	"testing"
)

func initTempAIAffinityDB(t *testing.T) {
	t.Helper()
	if err := Init(filepath.Join(t.TempDir(), "test.db")); err != nil {
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
