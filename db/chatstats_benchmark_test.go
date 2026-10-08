package db

import (
	"testing"
	"time"
)

func BenchmarkPostgresChatInsightQueries(b *testing.B) {
	initPostgresForTest(b)

	const (
		groupID      = int64(9_202_608_30)
		messageCount = 30_000
	)
	end := time.Now().Truncate(time.Second)
	start := end.Add(-30 * 24 * time.Hour)
	windowSeconds := int64(end.Sub(start) / time.Second)
	if err := DB.Exec(`
		INSERT INTO group_chat_messages
			(group_id, message_id, user_id, nickname, content, stat_excluded, created_at)
		SELECT ?, sequence,
			((sequence - 1) % 8) + 1,
			'群友' || (((sequence - 1) % 8) + 1)::text,
			CASE
				WHEN sequence % 11 = 0 THEN '今晚一起吃火锅'
				WHEN sequence % 7 = 0 THEN '这个功能真的很好用'
				WHEN sequence % 5 = 0 THEN '明天晚上一起打游戏'
				ELSE '群聊消息 ' || sequence::text || ' 项目进度和测试结果'
			END,
			false,
			? + ((sequence * 73) % ?)
		FROM generate_series(1, ?::integer) AS sequence`,
		groupID, start.Unix(), windowSeconds, messageCount).Error; err != nil {
		b.Fatalf("seed chat messages: %v", err)
	}

	userIDs := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	b.ReportAllocs()
	b.ReportMetric(5, "sql/op")
	b.ResetTimer()
	for range b.N {
		if _, err := GetGroupChatSummary(groupID, 0, start, end); err != nil {
			b.Fatal(err)
		}
		if _, err := GetGroupChatTopWords(groupID, 0, start, end, 256); err != nil {
			b.Fatal(err)
		}
		if _, err := GetGroupChatUserCounts(groupID, start, end, 8); err != nil {
			b.Fatal(err)
		}
		if _, err := GetGroupChatTopPhrasesForUsers(groupID, userIDs, start, end, 4); err != nil {
			b.Fatal(err)
		}
		if _, err := GetGroupChatTopWordsForUsers(groupID, userIDs, start, end, 96); err != nil {
			b.Fatal(err)
		}
	}
}
