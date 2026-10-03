package bilibili

import (
	"github.com/Yuelioi/yueling-go/bot"
	"slices"
	"strings"
	"testing"

	"github.com/Yuelioi/bpi-go/ids"
	"github.com/Yuelioi/bpi-go/video"
	bservice "github.com/Yuelioi/yueling-go/services/bilibili"
)

func TestRecipientsFallbackAndDeduplication(t *testing.T) {
	if got := recipients([]int64{99, 99, 98}, []int64{97}); !slices.Equal(got, []int64{99, 98}) {
		t.Fatal(got)
	}
	if got := recipients(nil, []int64{99, 99, 0}); !slices.Equal(got, []int64{99}) {
		t.Fatal(got)
	}
}

func TestCommandInputs(t *testing.T) {
	uid, count, err := parseRecentArgs([]string{"123", "3"})
	if err != nil || uid != 123 || count != 3 {
		t.Fatalf("recent=%d %d %v", uid, count, err)
	}
	if _, _, err := parseRecentArgs([]string{"123", "11"}); err == nil {
		t.Fatal("unbounded count")
	}
	if normalizeMode("关闭") != "off" || normalizeMode("AI") != "ai" || normalizeMode("混合") != "hybrid" {
		t.Fatal("reply mode aliases")
	}
}

func TestReplyPreservesParagraphs(t *testing.T) {
	old := bot.CmdPrefix
	bot.CmdPrefix = "/"
	t.Cleanup(func() { bot.CmdPrefix = old })
	if got := replyBody("/B站回复 1 你好\n第二行  world", "B站回复"); got != "1 你好\n第二行  world" {
		t.Fatal(got)
	}
}

func TestStatsRetainUnknownFavoriteAndPartScope(t *testing.T) {
	id, _ := ids.ParseBVID("BV1xx411c7mD")
	stats := bservice.VideoStats{View: video.View{BVID: id, Title: "example", Stat: video.Stat{View: 123}}, Part: 2, Online: video.OnlineTotal{Total: "12", Count: "4", ShowSwitch: video.OnlineTotalSwitch{Total: true, Count: true}}}
	got := formatStats(stats)
	for _, want := range []string{"当前在线（P2）：12（网页端 4）", "播放 123", "收藏 未知"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	stats.Online.ShowSwitch.Total = false
	stats.Online.ShowSwitch.Count = false
	if got := formatStats(stats); !strings.Contains(got, "暂不可用") {
		t.Fatal(got)
	}
}
