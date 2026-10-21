package ai

import (
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

func TestNewReferenceDoesNotForgetEarlierImages(t *testing.T) {
	s := newSession(1, 100)
	s.attachImages("白发角色", []string{"https://fixture.invalid/white.png"}, nil)
	s.attachImages("参考这张粉发图的动作", nil, []string{"https://fixture.invalid/pink.png"})
	urls, err := (&ToolContext{session: s}).ImageURLs([]string{"image_1", "image_2"})
	if err != nil || len(urls) != 2 || urls[0] != "https://fixture.invalid/white.png" {
		t.Fatalf("new quote erased the earlier reference: urls=%v err=%v", urls, err)
	}
	input := s.attachImages("刚才的白发角色", nil, nil)
	if !strings.Contains(input, "image_1") || !strings.Contains(input, "image_2") {
		t.Fatal("follow-up did not expose both distinguishable references")
	}
}

func TestImageIndexSearchFindsOutputsOutsideTheShortPrompt(t *testing.T) {
	s := newSession(1, 100)
	c := &ToolContext{session: s, event: &bot.GroupMessageEvent{GroupID: 100, UserID: 42}}
	id, err := c.GeneratedImage(123, "银白发Q版", "png")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxSessionImages-1; i++ {
		s.attachImages("新的参考", []string{"https://fixture.invalid/other.png"}, nil)
	}
	if strings.Contains(s.attachImages("继续聊", nil, nil), "银白发Q版") {
		t.Fatal("old image unexpectedly remained in the bounded prompt view")
	}
	result := c.ImageHistory("白发", 8)
	if !strings.Contains(result, id) || !strings.Contains(result, "银白发Q版") || strings.Contains(result, "https://") {
		t.Fatal("retained image could not be found without loading long chat history", result)
	}
	if strings.Contains((&ToolContext{session: newSession(1, 101), event: &bot.GroupMessageEvent{GroupID: 101, UserID: 42}}).ImageHistory("白发", 8), id) {
		t.Fatal("image index crossed group scope")
	}
}
