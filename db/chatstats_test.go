package db

import (
	"testing"
	"time"
)

func TestGroupChatMessagesAreIdempotentAndIsolated(t *testing.T) {
	initPostgresForTest(t)

	now := time.Unix(1_700_000_000, 0)
	rows := []GroupChatMessage{
		{GroupID: 100, MessageID: 1, UserID: 10, Nickname: "甲", Content: "今晚吃火锅", CreatedAt: now.Unix()},
		{GroupID: 100, MessageID: 2, UserID: 20, Nickname: "乙", Content: "火锅可以", CreatedAt: now.Add(time.Minute).Unix()},
		{GroupID: 200, MessageID: 1, UserID: 10, Nickname: "甲", Content: "另一个群", CreatedAt: now.Unix()},
	}
	if err := SaveGroupChatMessages(rows); err != nil {
		t.Fatal(err)
	}
	if err := SaveGroupChatMessage(rows[0]); err != nil {
		t.Fatalf("duplicate insert: %v", err)
	}

	got, err := GetGroupChatMessages(100, 0, now.Add(-time.Second), now.Add(time.Hour), 100)
	if err != nil || len(got) != 2 {
		t.Fatalf("group rows=%+v err=%v", got, err)
	}
	mine, err := GetGroupChatMessages(100, 20, now.Add(-time.Second), now.Add(time.Hour), 100)
	if err != nil || len(mine) != 1 || mine[0].Nickname != "乙" {
		t.Fatalf("user rows=%+v err=%v", mine, err)
	}
}

func TestDeleteOldGroupChatMessages(t *testing.T) {
	initPostgresForTest(t)

	now := time.Unix(1_700_000_000, 0)
	if err := SaveGroupChatMessages([]GroupChatMessage{
		{GroupID: 100, MessageID: 1, UserID: 10, CreatedAt: now.Add(-40 * 24 * time.Hour).Unix()},
		{GroupID: 100, MessageID: 2, UserID: 10, CreatedAt: now.Unix()},
	}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteGroupChatMessagesBefore(now.Add(-35 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := GetGroupChatMessages(100, 0, now.Add(-60*24*time.Hour), now.Add(time.Hour), 100)
	if err != nil || len(got) != 1 || got[0].MessageID != 2 {
		t.Fatalf("rows after retention=%+v err=%v", got, err)
	}
}
