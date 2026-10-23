package cards

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/wuxia"
)

func TestMain(m *testing.M) {
	services.DataDir = filepath.Join("..", "..", "..", "data")
	os.Exit(m.Run())
}

func TestLocalCardsRenderWithoutModel(t *testing.T) {
	c := wuxia.DefaultCatalog()
	p := wuxia.Character{GroupID: 1, UserID: 10, Name: "梦之咩咩团子", ProfessionID: "swordsman", Experience: 360, Silver: 286, WeaponKind: "剑", Weapons: map[string]string{"剑": "剑-2"}, Portrait: ""}
	v, err := c.View(p, wuxia.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	data, err := Profile(context.Background(), v, wuxia.Daily{Runs: 3}, 3)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != CanvasWidth || img.Bounds().Dy() != 1000 {
		t.Fatalf("profile: %v", err)
	}
	if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/profile.png", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	r := wuxia.Result{Actor: v, Daily: wuxia.Daily{Runs: 3, Signed: true}, DailyLimit: 3, SilverDelta: 30, ExperienceDelta: 12, Lines: []string{"签到 +30银两，+12阅历", "等级 3 → 4，武艺精进"}}
	p.Silver += 30
	p.Experience += 12
	for seed := range 3 {
		e, err := c.Explore(&p, wuxia.DefaultSettings(), uint64(seed+13))
		if err != nil {
			t.Fatal(err)
		}
		r.Encounters = append(r.Encounters, e)
		r.SilverDelta += e.Silver
		r.ExperienceDelta += e.Experience
	}
	r.Actor, err = c.View(p, wuxia.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	data, err = Report(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
		if err := os.WriteFile(dir+"/adventure.png", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	newcomer := wuxia.Character{Name: "初入江湖的咩咩", ProfessionID: "swordsman", Experience: 12, Silver: 30, WeaponKind: "剑", Weapons: map[string]string{"剑": "剑-1"}}
	fresh := wuxia.Result{Joined: true, SettledAt: time.Date(2026, 10, 22, 19, 0, 0, 0, time.FixedZone("CST", 8*3600)), Daily: wuxia.Daily{Runs: 3, Signed: true}, DailyLimit: 3, SilverDelta: 30, ExperienceDelta: 12, Lines: []string{"初入江湖 · 已创建剑客，发送「江湖转职」可免费选择职业", "签到 +30银两，+12阅历"}}
	for seed := range 3 {
		e, err := c.Explore(&newcomer, wuxia.DefaultSettings(), uint64(seed+13))
		if err != nil {
			t.Fatal(err)
		}
		fresh.Encounters = append(fresh.Encounters, e)
		fresh.SilverDelta += e.Silver
		fresh.ExperienceDelta += e.Experience
	}
	fresh.Actor, err = c.View(newcomer, wuxia.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	for _, replay := range []bool{false, true} {
		fresh.Replayed = replay
		data, err := Report(context.Background(), fresh)
		if err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
			name := "onboarding.png"
			if replay {
				name = "replay.png"
			}
			if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, count := range []int{0, 1, 2} {
		variant := r
		variant.Encounters = r.Encounters[:count]
		variant.SilverDelta = 30
		variant.ExperienceDelta = 12
		variant.Daily.Runs = count
		for _, e := range variant.Encounters {
			variant.SilverDelta += e.Silver
			variant.ExperienceDelta += e.Experience
		}
		if count == 0 {
			variant.SilverDelta = 0
			variant.ExperienceDelta = 0
			variant.Lines = []string{"兵刃已装备：青锋剑"}
		}
		data, err := Report(context.Background(), variant)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("adventure-%d.png", count)), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	a := v.Fighter()
	opponent, err := c.View(wuxia.Character{Name: "山雨刀客", ProfessionID: "blade", Experience: 360, WeaponKind: "刀", Weapons: map[string]string{"刀": "刀-2"}}, wuxia.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	battle := wuxia.Duel(a, opponent.Fighter(), 52)
	r.Battle = &battle
	data, err = Report(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
		if err := os.WriteFile(dir+"/duel.png", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Report(ctx, r); err == nil {
		t.Fatal("cancelled rendering continued")
	}
}

func TestGameSpritesKeepTrueTransparency(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ink-sprite-sword", "ink-sprite-blade", "ink-sprite-boxer", "ink-item-sword", "ink-item-blade", "ink-item-wraps", "ink-item-armor", "ink-item-jade", "ink-item-silver", "ink-item-scroll", "ink-item-pouch"} {
		t.Run(id, func(t *testing.T) {
			img := asset(id)
			b := img.Bounds()
			transparent, solid, total := 0, 0, 0
			for y := b.Min.Y; y < b.Max.Y; y += 4 {
				for x := b.Min.X; x < b.Max.X; x += 4 {
					_, _, _, a := img.At(x, y).RGBA()
					total++
					if a == 0 {
						transparent++
					}
					if a >= 62000 {
						solid++
					}
				}
			}
			if transparent < total/10 || solid < total/10 {
				t.Fatalf("asset is not a visible cutout: transparent=%d solid=%d total=%d", transparent, solid, total)
			}
		})
	}
}

func TestGameCardVariants(t *testing.T) {
	catalog := wuxia.DefaultCatalog()
	for _, prof := range catalog.Professions {
		t.Run(prof.ID, func(t *testing.T) {
			character := wuxia.Character{Name: "路过江湖的侠客", ProfessionID: prof.ID, Experience: 420, Silver: 9999999, WeaponKind: prof.WeaponKind, Weapons: map[string]string{prof.WeaponKind: catalog.Starter(prof.WeaponKind).ID}}
			v, err := catalog.View(character, wuxia.DefaultSettings())
			if err != nil {
				t.Fatal(err)
			}
			data, err := Profile(context.Background(), v, wuxia.Daily{}, 3)
			if err != nil {
				t.Fatal(err)
			}
			config, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil || config.Width != CanvasWidth || config.Height != 1000 {
				t.Fatalf("variant %v %+v", err, config)
			}
			if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, "profile-"+prof.ID+".png"), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			character.Name = "这是一个非常非常长的群聊侠客昵称用于验证布局边界"
			character.Portrait = "boxer"
			v, err = catalog.View(character, wuxia.DefaultSettings())
			if err != nil {
				t.Fatal(err)
			}
			r := wuxia.Result{Actor: v, Daily: wuxia.Daily{Runs: 12}, DailyLimit: 12, SilverDelta: 240, ExperienceDelta: 360, Lines: []string{"今日签到和全部历练均已结算"}}
			for i := range 12 {
				r.Encounters = append(r.Encounters, wuxia.Encounter{Scene: catalog.Scenes[i%len(catalog.Scenes)].ID, Title: "一场很长名字的江湖奇遇", Text: "在山间偶遇了远行的侠客，互道珍重以后继续前行。", Silver: 20, Experience: 30, Loot: "拾得青锋剑，已收入收藏"})
			}
			data, err = Report(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			config, err = png.DecodeConfig(bytes.NewReader(data))
			if err != nil || config.Width != CanvasWidth || config.Height < 1600 {
				t.Fatalf("daily limit %v %+v", err, config)
			}
			if prof.ID == "swordsman" {
				if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
					if err := os.WriteFile(filepath.Join(dir, "adventure-12.png"), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestDisplayFontCoversMenuLabels(t *testing.T) {
	f, err := loadDisplayFont()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range "属性侠客行囊兵刃收藏当前装备今日历练战力气血攻击防御身法武学踏入江湖研习切磋" {
		glyph, err := f.GlyphIndex(nil, r)
		if err != nil || glyph == 0 {
			t.Fatalf("menu glyph missing: %c %v", r, err)
		}
	}
}

func TestDuelReportShowsBothPlayersAndPreservesOldSnapshot(t *testing.T) {
	c := wuxia.DefaultCatalog()
	v, err := c.View(wuxia.Character{Name: "月离", ProfessionID: "swordsman", WeaponKind: "剑", Weapons: map[string]string{"剑": "剑-1"}}, wuxia.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	left, right := v.Fighter(), v.Fighter()
	right.Name = "野良"
	battle := wuxia.Duel(left, right, 24)
	r := wuxia.Result{Actor: v, Battle: &battle}
	summary := Summary(r)
	for _, name := range []string{"月离", "野良", "松纹剑", "气血"} {
		if !strings.Contains(summary, name) {
			t.Fatalf("summary missing %s", name)
		}
	}
	for _, text := range []string{"三分", "胜率", "无损", "不扣资源", "不发银两"} {
		if strings.Contains(summary, text) {
			t.Fatalf("debug copy in report: %s", text)
		}
	}
	for _, legacy := range []bool{false, true} {
		if legacy {
			r.Battle = &wuxia.Battle{Version: "duel-points-1", Left: left, Right: right, Winner: 1, Rounds: 3, Score: [2]int{0, 3}, Highlights: []string{"野良抢占先机，一招得分 · 0:1", "野良变招破防，拿下一分 · 0:2", "野良稳住架势，后发制人 · 0:3"}}
		}
		before, _ := json.Marshal(r.Battle)
		data, err := Report(context.Background(), r)
		if err != nil {
			t.Fatal(err)
		}
		after, _ := json.Marshal(r.Battle)
		if !bytes.Equal(before, after) {
			t.Fatal("render rewrote stored battle")
		}
		if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
			name := "duel-yueli.png"
			if legacy {
				name = "duel-legacy.png"
			}
			if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestInventoryAndDrawCards(t *testing.T) {
	c := wuxia.DefaultCatalog()
	p := wuxia.Character{Name: "月离", ProfessionID: "swordsman", WeaponKind: "剑", Weapons: map[string]string{"剑": "剑-1"}, Inventory: map[string]int64{"剑-1": 1}, Silver: 1000}
	if err := c.Grant(&p, []wuxia.Reward{{ItemID: "armor-cloth", Quantity: 1}, {ItemID: "charm-jade", Quantity: 1}, {ItemID: "manual-sword-rain", Quantity: 1}}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"armor-cloth", "charm-jade"} {
		if err := c.Equip(&p, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Learn(&p, "manual-sword-rain"); err != nil {
		t.Fatal(err)
	}
	rewards, err := c.Draw(&p, "jianghu", 5, 31)
	if err != nil {
		t.Fatal(err)
	}
	v, err := c.View(p, wuxia.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	result := wuxia.Result{Actor: v, DailyLimit: 3, SilverDelta: -300, DrawPool: "jianghu", Rewards: rewards, Lines: []string{"江湖宝匣 · 寻宝所得"}}
	for _, reward := range rewards {
		item, _ := c.Item(reward.ItemID)
		result.RewardItems = append(result.RewardItems, item)
		result.Lines = append(result.Lines, fmt.Sprintf("%s ×%d", item.Name, reward.Quantity))
	}
	for _, kind := range []string{"inventory-profile", "draw-five"} {
		var data []byte
		if kind == "draw-five" {
			data, err = Report(context.Background(), result)
		} else {
			data, err = Profile(context.Background(), v, wuxia.Daily{}, 3)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, kind+".png"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestUpgradeProfessionPreviews(t *testing.T) {
	c := wuxia.DefaultCatalog()
	for _, prof := range c.Professions {
		p := wuxia.Character{Name: prof.Name + " · 月离", ProfessionID: prof.ID, Experience: wuxia.XPForLevel(10), WeaponKind: prof.WeaponKind, Weapons: map[string]string{prof.WeaponKind: prof.WeaponKind + "-4"}, Inventory: map[string]int64{prof.WeaponKind + "-4": 1}, Silver: 1280}
		v, err := c.View(p, wuxia.DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		data, err := Profile(context.Background(), v, wuxia.Daily{Signed: true, Runs: 3}, 3)
		if err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "upgrade-"+prof.ID+".png"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPlayerSummaryDoesNotExposeReceiptLanguage(t *testing.T) {
	for _, replay := range []bool{false, true} {
		text := Summary(wuxia.Result{Replayed: replay, DailyLimit: 3})
		for _, forbidden := range []string{"操作已保存", "原操作回执", "奖励不重复领取"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("player summary contains %q", forbidden)
			}
		}
		if replay && !strings.Contains(text, "旧事重温") {
			t.Fatal("replayed summary must distinguish the earlier journey")
		}
	}
}

func TestEquipmentArtHasTransparentBackground(t *testing.T) {
	if err := loadEquipmentArt(); err != nil {
		t.Fatal(err)
	}
	for name, img := range equipmentArt.images {
		b := img.Bounds()
		transparent, solid, total := 0, 0, 0
		for y := b.Min.Y; y < b.Max.Y; y += 8 {
			for x := b.Min.X; x < b.Max.X; x += 8 {
				_, _, _, a := img.At(x, y).RGBA()
				total++
				if a == 0 {
					transparent++
				}
				if a > 60000 {
					solid++
				}
			}
		}
		if transparent < total/5 || solid < total/100 {
			t.Errorf("%s: expected isolated equipment with transparent surroundings, transparent=%d solid=%d total=%d", name, transparent, solid, total)
		}
	}
}

func TestBattleFlavorMatchesOutcome(t *testing.T) {
	for _, tc := range []struct {
		name       string
		winner, hp int
		recorded   bool
		want       string
	}{
		{"draw", -1, 100, true, "不分伯仲"},
		{"untouched", 0, 100, true, "完胜"},
		{"dominant", 0, 80, true, "稳稳拿下"},
		{"close", 1, 20, true, "险胜"},
		{"regular", 1, 50, true, "胜一筹"},
		{"legacy", 0, 0, false, "胜一筹"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := wuxia.Battle{Winner: tc.winner, Left: wuxia.Fighter{Stats: wuxia.Stats{HP: 100}}, Right: wuxia.Fighter{Stats: wuxia.Stats{HP: 100}}, Remaining: [2]int{tc.hp, tc.hp}}
			if tc.recorded {
				b.Actions = []wuxia.CombatAction{{}}
			}
			label, quip := battleFlavor(&b)
			if label != tc.want || quip == "" {
				t.Fatalf("got %q %q, want %q", label, quip, tc.want)
			}
		})
	}
}

func TestBattleBanterVariesButReplayIsStable(t *testing.T) {
	b := wuxia.Battle{Winner: 0, Left: wuxia.Fighter{Stats: wuxia.Stats{HP: 100}}, Remaining: [2]int{100, 0}, Actions: []wuxia.CombatAction{{}}}
	seen := map[string]bool{}
	for seed := uint64(0); seed < 8; seed++ {
		b.Seed = seed
		label, line := battleFlavor(&b)
		again, replay := battleFlavor(&b)
		if label != "完胜" || again != label || replay != line {
			t.Fatal("replay changed outcome or banter")
		}
		seen[line] = true
	}
	if len(seen) != 8 {
		t.Fatalf("expected eight distinct full-health victory lines, got %d", len(seen))
	}
}

func TestBattleTrailTurnsWithoutChangingOrder(t *testing.T) {
	want := [][2]int{{180, 823}, {455, 823}, {730, 823}, {1005, 823}, {1005, 943}, {730, 943}, {455, 943}, {180, 943}, {180, 1063}}
	for n, point := range want {
		x, y := battleTrailPoint(n)
		if x != point[0] || y != point[1] {
			t.Fatalf("action %d: (%d,%d), want %v", n, x, y, point)
		}
	}
}

func TestAdventureVarietyPreview(t *testing.T) {
	c := wuxia.DefaultCatalog()
	p := wuxia.Character{Name: "梦之咩咩团子", ProfessionID: "swordsman", WeaponKind: "剑", Weapons: map[string]string{"剑": c.Starter("剑").ID}, Silver: 100, Experience: wuxia.XPForLevel(4)}
	r := wuxia.Result{DailyLimit: 3, Daily: wuxia.Daily{Runs: 3, Signed: true}}
	for n, id := range []string{"mentor", "purse", "pickpocket"} {
		selected := c
		for _, e := range c.Events {
			if e.ID == id {
				selected.Events = []wuxia.Event{e}
				break
			}
		}
		encounter, err := selected.Explore(&p, wuxia.DefaultSettings(), uint64(n+1))
		if err != nil {
			t.Fatal(err)
		}
		r.Encounters = append(r.Encounters, encounter)
		r.SilverDelta += encounter.Silver
		r.ExperienceDelta += encounter.Experience
	}
	var err error
	r.Actor, err = c.View(p, wuxia.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	data, err := Report(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "adventure-variety.png"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCasinoCardPreviews(t *testing.T) {
	c := wuxia.DefaultCatalog()
	for _, tc := range []struct {
		name string
		dice [3]int
	}{{"casino-win", [3]int{4, 5, 6}}, {"casino-lose", [3]int{1, 2, 5}}, {"casino-triple", [3]int{6, 6, 6}}, {"casino-half", [3]int{1, 3, 5}}, {"casino-pair", [3]int{2, 2, 5}}} {
		wager, err := wuxia.ResolveCasino(100, tc.dice)
		if err != nil {
			t.Fatal(err)
		}
		v, err := c.View(wuxia.Character{Name: "梦之咩咩团子", ProfessionID: "swordsman", WeaponKind: "剑", Weapons: map[string]string{"剑": c.Starter("剑").ID}, Silver: 500 + wager.Payout - wager.Stake}, wuxia.DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		data, err := Report(context.Background(), wuxia.Result{Actor: v, Casino: &wager})
		if err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, tc.name+".png"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
