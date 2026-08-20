package funny

import (
	"bytes"
	"image"
	"image/jpeg"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services"
)

func TestChatStatsPeriod(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Date(2026, 8, 13, 15, 30, 0, 0, loc)
	label, start, end := chatStatsPeriod("昨日词云", now)
	if label != "昨日" || start.Format("2006-01-02 15:04") != "2026-08-12 00:00" || end.Format("2006-01-02 15:04") != "2026-08-13 00:00" {
		t.Fatalf("yesterday = %q %v %v", label, start, end)
	}
	label, start, end = chatStatsPeriod("本周废话榜", now)
	if label != "本周" || start.Format("2006-01-02") != "2026-08-10" || end.Format("2006-01-02") != "2026-08-14" {
		t.Fatalf("week = %q %v %v", label, start, end)
	}
}

func TestSelectChatWordsFiltersStopWordsAndRedundantTerms(t *testing.T) {
	words := selectChatWords([]db.GroupChatWordCount{
		{Text: "火锅", Count: 8},
		{Text: "重庆火锅", Count: 7},
		{Text: "可以", Count: 20},
	}, 20, 8)
	if len(words) != 1 || words[0].Text != "重庆火锅" || words[0].Count != 7 {
		t.Fatalf("words = %+v", words)
	}
}

func TestChatAnalysisFromDatabaseUsesPostgresAggregates(t *testing.T) {
	got := chatAnalysisFromDatabase("今日", db.GroupChatSummary{Total: 3, TextTotal: 3, Participants: 2},
		[]db.GroupChatWordCount{{Text: "火锅", Count: 3}, {Text: "可以", Count: 2}},
		[]db.GroupChatUserCount{{UserID: 1, Nickname: "甲", Count: 2}, {UserID: 2, Nickname: "乙", Count: 1}})
	if got.Total != 3 || got.Participants != 2 || len(got.Users) != 2 {
		t.Fatalf("analysis = %+v", got)
	}
	if got.Users[0].Nickname != "甲" || got.Users[0].Count != 2 {
		t.Fatalf("leader = %+v", got.Users[0])
	}
	if len(got.Words) != 1 || got.Words[0].Text != "火锅" || got.Words[0].Count != 3 {
		t.Fatalf("words = %+v, missing 火锅 count 3", got.Words)
	}
}

func TestChatStatsCommandDetection(t *testing.T) {
	if !isChatStatsCommand("/谁最爱说 火锅") || !isChatStatsCommand("今日词云") || isChatStatsCommand("今天聊聊词云实现") {
		t.Fatal("chat stats command detection mismatch")
	}
}

func TestRenderChatWordCloud(t *testing.T) {
	oldDataDir := services.DataDir
	services.DataDir = filepath.Join("..", "..", "data")
	chatCloudFontOnce = sync.Once{}
	chatCloudFont = nil
	chatCloudFontErr = nil
	t.Cleanup(func() {
		services.DataDir = oldDataDir
		chatCloudFontOnce = sync.Once{}
		chatCloudFont = nil
		chatCloudFontErr = nil
	})

	words := []chatWord{
		{Text: "火锅", Count: 20}, {Text: "群友", Count: 18}, {Text: "Steam史低", Count: 15},
		{Text: "猫猫", Count: 12}, {Text: "下班", Count: 10}, {Text: "表情包", Count: 9},
		{Text: "好耶", Count: 8}, {Text: "周末", Count: 7}, {Text: "开黑", Count: 6},
	}
	data, err := renderChatWordCloud(chatAnalysis{Label: "今日", Total: 128, Participants: 16, Words: words})
	if err != nil {
		t.Fatal(err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != chatCloudWidth || config.Height != chatCloudHeight {
		t.Fatalf("rendered size = %dx%d", config.Width, config.Height)
	}
}

func TestChatWordCloudLayoutKeepsWordsApart(t *testing.T) {
	oldDataDir := services.DataDir
	services.DataDir = filepath.Join("..", "..", "data")
	chatCloudFontOnce = sync.Once{}
	chatCloudFont = nil
	chatCloudFontErr = nil
	t.Cleanup(func() {
		services.DataDir = oldDataDir
		chatCloudFontOnce = sync.Once{}
		chatCloudFont = nil
		chatCloudFontErr = nil
	})

	parsed, err := loadChatCloudFont()
	if err != nil {
		t.Fatal(err)
	}
	texts := []string{
		"消息", "头疼", "灵活", "选择", "想题", "放假", "大部分", "抽签",
		"周末", "开黑", "火锅", "猫猫", "表情包", "好耶", "群友", "下班",
		"电影", "游戏", "天气", "晚饭", "作业", "考试", "摸鱼", "睡觉",
		"咖啡", "奶茶", "旅行", "音乐", "照片", "代码", "更新", "机器人",
	}
	words := make([]chatWord, len(texts))
	for i, text := range texts {
		words[i] = chatWord{Text: text, Count: len(texts) - i + 2}
	}
	placements := layoutChatCloudWords(parsed, words, rand.New(rand.NewSource(7)))
	if len(placements) < len(words)-2 {
		t.Fatalf("placed only %d of %d words", len(placements), len(words))
	}
	for i, placement := range placements {
		bounds := placement.sprite.collision.Bounds().Add(placement.at)
		if !bounds.In(chatCloudWordArea) {
			t.Fatalf("word %q outside cloud area: %v", placement.text, bounds)
		}
		for j := 0; j < i; j++ {
			if chatCloudPlacementMasksOverlap(placement, placements[j]) {
				t.Fatalf("words %q and %q overlap", placement.text, placements[j].text)
			}
		}
	}
}

func chatCloudPlacementMasksOverlap(a, b chatCloudPlacement) bool {
	overlap := a.sprite.collision.Bounds().Add(a.at).Intersect(b.sprite.collision.Bounds().Add(b.at))
	for y := overlap.Min.Y; y < overlap.Max.Y; y++ {
		for x := overlap.Min.X; x < overlap.Max.X; x++ {
			point := image.Pt(x, y)
			if a.sprite.collision.AlphaAt(point.X-a.at.X, point.Y-a.at.Y).A != 0 &&
				b.sprite.collision.AlphaAt(point.X-b.at.X, point.Y-b.at.Y).A != 0 {
				return true
			}
		}
	}
	return false
}
