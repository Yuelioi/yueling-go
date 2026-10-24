package cards

import (
	"bytes"
	"context"
	"github.com/Yuelioi/yueling-go/services/wuxia"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectionPages(t *testing.T) {
	raw, err := os.ReadFile("../internal/content/six_professions.json")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := wuxia.ParseCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows := []ListEntry{}
	for _, item := range catalog.Items {
		if item.Image != "" && len(rows) < 7 {
			rows = append(rows, ListEntry{Name: item.Name, Detail: item.Description, Tier: item.Tier, ArtName: item.Name, Badge: "已收藏"})
		}
	}
	pages, err := Collection(context.Background(), "江湖行囊", "江湖装备 物品名 · 江湖学艺 秘籍名", rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("seven items: got %d pages", len(pages))
	}
	for _, data := range pages {
		im, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if im.Bounds().Dx() != 1200 {
			t.Fatal(im.Bounds())
		}
	}
	save := func(name string, pages [][]byte, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+".png"), pages[0], 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	save("candidate-inventory", pages, nil)
	pages, err = Collection(context.Background(), "江湖行囊", "江湖寻宝 · 寻访机缘", nil)
	save("candidate-bag-empty", pages, err)
	rows = nil
	for _, s := range catalog.Skills {
		if s.Kind == "mind" && len(rows) < 5 {
			rows = append(rows, ListEntry{Name: s.Name, Detail: s.RuleText(), SkillID: s.ID, Tier: s.Tier, Badge: "已学"})
		}
	}
	pages, err = Collection(context.Background(), "修习心法", "江湖心法 名称 · 江湖心法 卸下", rows)
	save("candidate-mind-list", pages, err)
	pages, err = Notice(context.Background(), "江湖手帖", "历练：江湖历练 · 一次完成今日剩余三次历练\n切磋：切磋 @群友 · 每日首次主动切磋领取银两与阅历\n寻宝：江湖寻宝 · 寻访兵刃、装备与武学秘籍\n赌场：江湖赌场 · 默认赌金100，直接开奖\n行囊：江湖背包 · 江湖装备 物品名\n武学：江湖武技 名称 · 江湖心法 名称")
	save("candidate-help", pages, err)
	rows = nil
	for _, a := range catalog.Appearances {
		rows = append(rows, ListEntry{Name: a.Name, Portrait: a.ID})
	}
	if _, err := Collection(context.Background(), "侠客风貌", "江湖形象 编号", rows); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Collection(ctx, "test", "", nil); err == nil {
		t.Fatal("cancelled render succeeded")
	}
}
