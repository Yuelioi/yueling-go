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

func TestSelectGroupChatWordsFiltersGenericAndRedundantTerms(t *testing.T) {
	words := SelectGroupChatWords([]GroupChatWordCount{
		{Text: "火锅", Count: 8},
		{Text: "重庆火锅", Count: 7},
		{Text: "可以", Count: 20},
	}, 20, 8)
	if len(words) != 1 || words[0].Text != "重庆火锅" || words[0].Count != 7 {
		t.Fatalf("words = %+v", words)
	}
}

func TestGroupChatUserInsightBatchesHandleNoUsers(t *testing.T) {
	words, err := GetGroupChatTopWordsForUsers(100, nil, time.Time{}, time.Time{}, 5)
	if err != nil || len(words) != 0 {
		t.Fatalf("words=%+v err=%v", words, err)
	}
	phrases, err := GetGroupChatTopPhrasesForUsers(100, nil, time.Time{}, time.Time{}, 5)
	if err != nil || len(phrases) != 0 {
		t.Fatalf("phrases=%+v err=%v", phrases, err)
	}
}

func TestGetGroupChatTopPhrasesReturnsRepeatedRealMessages(t *testing.T) {
	initPostgresForTest(t)

	now := time.Unix(1_700_000_000, 0)
	if err := SaveGroupChatMessages([]GroupChatMessage{
		{GroupID: 100, MessageID: 1, UserID: 10, Content: "好耶", CreatedAt: now.Unix()},
		{GroupID: 100, MessageID: 2, UserID: 10, Content: "好耶", CreatedAt: now.Add(time.Minute).Unix()},
		{GroupID: 100, MessageID: 3, UserID: 10, Content: "只说一次", CreatedAt: now.Add(2 * time.Minute).Unix()},
		{GroupID: 100, MessageID: 4, UserID: 20, Content: "好耶", CreatedAt: now.Add(3 * time.Minute).Unix()},
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := GetGroupChatTopPhrases(100, 10, now.Add(-time.Second), now.Add(time.Hour), 4)
	if err != nil || len(rows) != 1 || rows[0].Text != "好耶" || rows[0].Count != 2 {
		t.Fatalf("phrases=%+v err=%v", rows, err)
	}
	batched, err := GetGroupChatTopPhrasesForUsers(100, []int64{10, 20, 30}, now.Add(-time.Second), now.Add(time.Hour), 4)
	if err != nil || len(batched[10]) != 1 || batched[10][0].Text != "好耶" || batched[10][0].Count != 2 {
		t.Fatalf("batched phrases=%+v err=%v", batched, err)
	}
	if batched[20] == nil || len(batched[20]) != 0 || batched[30] == nil || len(batched[30]) != 0 {
		t.Fatalf("empty batched phrases=%+v", batched)
	}
}

func TestGroupChatHistoryStatsAndDeleteStayGroupScoped(t *testing.T) {
	initPostgresForTest(t)

	now := time.Unix(1_700_000_000, 0)
	if err := SaveGroupChatMessages([]GroupChatMessage{
		{GroupID: 100, MessageID: 1, UserID: 10, Content: "旧消息", CreatedAt: now.Add(-100 * 24 * time.Hour).Unix()},
		{GroupID: 100, MessageID: 2, UserID: 10, Content: "新消息", CreatedAt: now.Unix()},
		{GroupID: 200, MessageID: 1, UserID: 20, Content: "其他群", CreatedAt: now.Add(-100 * 24 * time.Hour).Unix()},
	}); err != nil {
		t.Fatal(err)
	}

	stats, err := GetGroupChatHistoryStats(100, now.Add(-90*24*time.Hour).Unix())
	if err != nil || stats.Total != 2 || stats.Matched != 1 || stats.OldestAt == 0 || stats.NewestAt == 0 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	deleted, err := DeleteGroupChatMessagesForGroupBefore(100, now.Add(-90*24*time.Hour).Unix())
	if err != nil || deleted != 1 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	if err := SaveGroupChatBackfill([]GroupChatMessage{
		{GroupID: 100, MessageID: 4, UserID: 10, Content: "补取旧消息", CreatedAt: now.Add(-100 * 24 * time.Hour).Unix()},
	}); err != nil {
		t.Fatal(err)
	}
	afterPartial, err := GetGroupChatHistoryStats(100, 0)
	if err != nil || afterPartial.Total != 1 {
		t.Fatalf("partial cleanup restored history=%+v err=%v", afterPartial, err)
	}
	deleted, err = DeleteAllGroupChatMessagesForGroup(100, now)
	if err != nil || deleted != 1 {
		t.Fatalf("delete all=%d err=%v", deleted, err)
	}
	other, err := GetGroupChatHistoryStats(200, 0)
	if err != nil || other.Total != 1 {
		t.Fatalf("other group stats=%+v err=%v", other, err)
	}
}

func TestDeletedGroupChatHistoryDoesNotReturnThroughBackfill(t *testing.T) {
	initPostgresForTest(t)

	now := time.Unix(1_700_000_000, 0)
	rows := []GroupChatMessage{
		{GroupID: 100, MessageID: 1, UserID: 10, Content: "旧消息一", CreatedAt: now.Add(-time.Hour).Unix()},
		{GroupID: 100, MessageID: 2, UserID: 20, Content: "旧消息二", CreatedAt: now.Unix()},
	}
	if err := SaveGroupChatMessages(rows); err != nil {
		t.Fatal(err)
	}
	deleted, err := DeleteAllGroupChatMessagesForGroup(100, now)
	if err != nil || deleted != 2 {
		t.Fatalf("delete all=%d err=%v", deleted, err)
	}
	if err := SaveGroupChatBackfill(rows); err != nil {
		t.Fatal(err)
	}
	stats, err := GetGroupChatHistoryStats(100, 0)
	if err != nil || stats.Total != 0 {
		t.Fatalf("restored stats=%+v err=%v", stats, err)
	}
	if err := SaveGroupChatMessage(GroupChatMessage{
		GroupID: 100, MessageID: 3, UserID: 10, Content: "删除后的新消息", CreatedAt: now.Add(time.Second).Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	stats, err = GetGroupChatHistoryStats(100, 0)
	if err != nil || stats.Total != 1 {
		t.Fatalf("new live stats=%+v err=%v", stats, err)
	}
}
