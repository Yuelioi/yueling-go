package wuxia

import (
	"context"
	"encoding/json"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/pgtest"
	"reflect"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/services/wuxia/internal/progression"
)

func upgradePlayer(c Catalog, profession Profession, level int) Character {
	w := c.Starter(profession.WeaponKind)
	return Character{Name: profession.Name, ProfessionID: profession.ID, Experience: XPForLevel(level), WeaponKind: w.Kind, Weapons: map[string]string{w.Kind: w.ID}, Inventory: map[string]int64{w.ID: 1}, Silver: 100000}
}

func TestUpgradeDrawEligibilityEveryLevelAndProfession(t *testing.T) {
	c := DefaultCatalog()
	for _, profession := range c.Professions {
		for level := 1; level <= c.MaxLevel; level++ {
			pool, _ := c.Pool("jianghu")
			possible := map[string]bool{}
			for _, e := range c.EligibleEntries(pool, profession.ID, level) {
				possible[e.Reward.ItemID] = true
			}

			for seed := range 500 {
				p := upgradePlayer(c, profession, level)
				rewards, err := c.Draw(&p, "jianghu", 5, uint64(seed))
				if err != nil {
					t.Fatal(err)
				}
				rebate := int64(0)
				for _, r := range rewards {
					rebate += r.ConvertedSilver
				}
				if p.Silver != 99700+rebate {
					t.Fatal("wrong cost")
				}
				for _, r := range rewards {
					if !possible[r.ItemID] {
						t.Fatalf("%s level %d got illegal reward %s", profession.ID, level, r.ItemID)
					}

				}
			}
		}
	}
}

func TestUpgradeNoEligibleRewardsIsAtomic(t *testing.T) {
	c := DefaultCatalog()
	c.Pools[0].Entries = []DrawEntry{{Reward: Reward{ItemID: "刀-4", Quantity: 1}, Weight: 1}}
	p := upgradePlayer(c, c.Professions[0], 1)
	before, _ := json.Marshal(p)
	if _, err := c.Draw(&p, "jianghu", 5, 1); err == nil {
		t.Fatal("empty filtered pool accepted")
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("failed draw changed player")
	}
}

func TestUpgradeEquipmentRestrictionsAndExplicitAutoEquip(t *testing.T) {
	c := DefaultCatalog()
	p := upgradePlayer(c, c.Professions[0], 1)
	if err := c.Grant(&p, []Reward{{ItemID: "剑-4", Quantity: 1}, {ItemID: "刀-1", Quantity: 1}, {ItemID: "剑-fine", Quantity: 1}, {ItemID: "armor-cloth", Quantity: 1}}); err != nil {
		t.Fatal(err)
	}
	if p.Weapons["剑"] != "剑-1" || len(p.Equipment) > 0 {
		t.Fatal("grant changed equipment")
	}
	for _, id := range []string{"刀-1"} {
		if c.Equip(&p, id) == nil {
			t.Fatalf("illegal equip: %s", id)
		}
	}
	c.AutoEquip(&p)
	if p.Weapons["剑"] != "剑-4" || p.Equipment["armor"] != "armor-cloth" {
		t.Fatal("empty armor slot was not filled or weapon was silently replaced")
	}
	p.Experience = XPForLevel(10)
	c.AutoEquip(&p)
	if p.Weapons["剑"] != "剑-4" || p.Inventory["剑-fine"] != 1 {
		t.Fatal("level unlock lost old item")
	}
	p.WeaponKind = "刀"
	p.Weapons["刀"] = "刀-1"
	if _, err := c.View(p, DefaultSettings()); err == nil {
		t.Fatal("illegal weapon affected battle snapshot")
	}
}

func TestUpgradeLevelCapAndProfessionGrowth(t *testing.T) {
	c := DefaultCatalog()
	for xp := int64(0); xp <= XPForLevel(15); xp++ {
		if Level(xp) > 10 {
			t.Fatal("level cap exceeded")
		}
	}
	if progression.Gain(XPForLevel(10)-2, 100, c.MaxLevel) != 2 || progression.Gain(XPForLevel(20), 100, c.MaxLevel) != 0 {
		t.Fatal("cap does not limit newly earned XP")
	}
	views := map[string]View{}
	for _, prof := range c.Professions {
		p := upgradePlayer(c, prof, 10)
		v, err := c.View(p, DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		views[prof.ID] = v
		r, err := c.Explore(&p, DefaultSettings(), 5)
		if err != nil || r.Experience != 0 || p.Experience != XPForLevel(10) {
			t.Fatal("capped adventure still earns XP")
		}
	}
	if views["swordsman"].Stats.Speed <= views["blade"].Stats.Speed || views["blade"].Stats.Attack <= views["boxer"].Stats.Attack || views["boxer"].Stats.HP <= views["swordsman"].Stats.HP || views["boxer"].Stats.Defense <= views["blade"].Stats.Defense {
		t.Fatal("profession identities lost")
	}
	for _, left := range views {
		for _, right := range views {
			p := DuelWinProbability(left.Fighter(), right.Fighter())
			if p < .1 || p > .9 {
				t.Fatal("win bounds violated")
			}
			wins := 0
			for seed := range 1000 {
				if Duel(left.Fighter(), right.Fighter(), uint64(seed)).Winner == 0 {
					wins++
				}
			}
			t.Logf("attribute reference %.3f, observed %.3f", p, float64(wins)/1000)
		}
	}
}

func TestUpgradeProfessionSwitchAndCappedSignIn(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	req := Request{BotID: 1, GroupID: 850, UserID: 10, MessageID: 1, Name: "拳师", Action: "daily"}
	if _, err := s.Operate(ctx, req); err != nil {
		t.Fatal(err)
	}
	req.MessageID++
	req.Action = "profession"
	req.Argument = "镇岳"
	r, err := s.Operate(ctx, req)
	if err != nil || r.Actor.Weapon.Kind != "拳套" {
		t.Fatalf("switch: %v %+v", err, r.Actor)
	}
	req.MessageID++
	req.Action = "weapon"
	req.Argument = "剑"
	if _, err := s.Operate(ctx, req); err == nil {
		t.Fatal("kind command bypasses profession restriction")
	}
	req.MessageID++
	req.Action = "profession"
	req.Argument = "问剑"
	r, err = s.Operate(ctx, req)
	if err != nil || r.Actor.Weapon.Kind != "剑" || r.Actor.Character.Inventory["拳套-1"] != 1 {
		t.Fatal("switch lost collection")
	}
	if err := s.database.Model(&Character{}).Where("group_id=? AND user_id=?", 850, 10).Update("experience", XPForLevel(10)).Error; err != nil {
		t.Fatal(err)
	}
	s.now = func() (now time.Time) { return time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC) }
	req.MessageID++
	req.Action = "daily"
	req.Argument = ""
	r, err = s.Operate(ctx, req)
	if err != nil || r.ExperienceDelta != 0 || r.Daily.Runs != 3 || r.Actor.Level != 10 {
		t.Fatalf("capped adventure: %v %+v", err, r)
	}
}

func TestUpgradeReceiptUsesStoredRewardSnapshot(t *testing.T) {
	c := DefaultCatalog()
	p := upgradePlayer(c, c.Professions[0], 1)
	rewards, err := c.Draw(&p, "jianghu", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := c.Item(rewards[0].ItemID)
	r := Result{Version: c.Version, Rewards: rewards, RewardItems: []Item{item}, DrawPoolName: "旧宝匣"}
	data, _ := json.Marshal(r)
	got, err := decodeReceipt(string(data))
	if err != nil || !reflect.DeepEqual(got.RewardItems, r.RewardItems) || got.DrawPoolName != "旧宝匣" {
		t.Fatal("snapshot not preserved")
	}
	legacy := `{"version":"wuxia-3","draw_pool":"jianghu","rewards":[{"item_id":"刀-1","quantity":1}]}`
	old, err := decodeReceipt(legacy)
	if err != nil || old.RewardItems[0].Stats.Attack != 6 {
		t.Fatal("old receipt used new weapon stats")
	}
}

func TestUpgradeMigrationPreservesAssetsAndRepairsIllegalLoadout(t *testing.T) {
	dsn := pgtest.NewSchema(t)
	cfg := config.DatabaseConfig{DSN: dsn, MaxOpen: 2, MaxIdle: 1, ConnMaxLifetime: 5}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	defer sqlDB.Close()
	if err := database.Exec(`DELETE FROM schema_migrations WHERE version=22;
 INSERT INTO wuxia_characters (group_id,user_id,name,profession_id,experience,silver,mastery,weapon_kind,weapons,inventory,created_at,updated_at)
 VALUES (860,10,'旧剑客','swordsman',0,123,0,'刀','{"刀":"刀-1","剑":"剑-3"}','{"刀-1":2,"剑-3":1}',now(),now()),
 (860,11,'老拳师','boxer',10000,456,12,'剑','{"剑":"剑-3","拳套":"拳套-3"}','{"剑-3":1,"拳套-3":2}',now(),now())`).Error; err != nil {
		t.Fatal(err)
	}
	upgraded, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlUpgraded, _ := upgraded.DB()
	defer sqlUpgraded.Close()
	s := New(upgraded)
	first, _, err := s.Profile(context.Background(), 860, 10)
	if err != nil || first.Weapon.ID != "剑-1" || first.Character.Inventory["刀-1"] != 2 || first.Character.Inventory["剑-3"] != 1 || first.Character.Silver != 123 {
		t.Fatalf("migration changed assets: %v %+v", err, first)
	}
	old, _, err := s.Profile(context.Background(), 860, 11)
	if err != nil || old.Level != 10 || old.Character.Experience != 10000 || old.Character.Mastery != 12 || old.Character.Silver != 456 || old.Weapon.ID != "拳套-3" {
		t.Fatalf("legacy progress lost: %v %+v", err, old)
	}
}
