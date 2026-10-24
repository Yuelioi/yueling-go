package wuxia

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/pgtest"
)

func TestInventoryEquipmentAndLearning(t *testing.T) {
	c := DefaultCatalog()
	p := Character{Name: "游侠", ProfessionID: "swordsman", WeaponKind: "剑", Weapons: map[string]string{"剑": "剑-1"}, Inventory: map[string]int64{"剑-1": 1}, Silver: 1000}
	before, err := c.View(p, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Grant(&p, []Reward{{ItemID: "剑-1", Quantity: 2}, {ItemID: "armor-cloth", Quantity: 1}, {ItemID: "charm-jade", Quantity: 1}, {ItemID: "manual-sword-rain", Quantity: 2}, {ItemID: "armor-leather", Quantity: 1}}); err != nil {
		t.Fatal(err)
	}
	c.AutoEquip(&p)
	if p.Inventory["剑-1"] != 1 || p.Inventory["manual-sword-rain"] != 1 || p.Equipment["armor"] != "armor-cloth" {
		t.Fatalf("lost rewards or equipped locked gear: %+v", p)
	}
	after, err := c.View(p, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if after.Stats.Defense != before.Stats.Defense+4 || after.Stats.HP != before.Stats.HP+100 || after.Stats.Speed != before.Stats.Speed+4 {
		t.Fatal("equipment bonuses not applied")
	}
	if err := c.Equip(&p, "玄纹皮甲"); err == nil {
		t.Fatal("level restriction bypassed")
	}
	if err := c.Learn(&p, "骤雨剑诀秘籍"); err != nil {
		t.Fatal(err)
	}
	after, err = c.View(p, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	if after.Skill == nil || after.Fighter().Techniques != after.Weapon.Techniques || after.Fighter().Martial == nil || after.Fighter().Martial.Hits != 2 || p.Inventory["manual-sword-rain"] != 0 {
		t.Fatal("learned skill has no effect")
	}
	if err := c.Learn(&p, "骤雨剑诀秘籍"); err == nil || p.Inventory["manual-sword-rain"] != 0 {
		t.Fatal("duplicate learning consumed a book")
	}
	if err := c.Grant(&p, []Reward{{ItemID: "刀-1", Quantity: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Equip(&p, "刀-1"); err == nil {
		t.Fatal("cross-profession weapon accepted")
	}
	p.ProfessionID = "blade"
	if err := c.Equip(&p, "刀-1"); err != nil {
		t.Fatal(err)
	}
	after, err = c.View(p, DefaultSettings())
	if err != nil || after.Skill != nil {
		t.Fatal("sword skill leaked into blade")
	}
	snapshot, _ := json.Marshal(p)
	if err := c.Grant(&p, []Reward{{ItemID: "剑-1", Quantity: 1}, {ItemID: "missing", Quantity: 1}}); err == nil {
		t.Fatal("bad reward accepted")
	}
	unchanged, _ := json.Marshal(p)
	if string(snapshot) != string(unchanged) {
		t.Fatal("failed grant mutated inventory")
	}
}

func TestDrawAtomicCostsReplayAndIsolation(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	req := Request{BotID: 1, GroupID: 600, UserID: 10, MessageID: 1, Name: "寻宝侠客", Action: "daily"}
	if _, err := s.Operate(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := s.database.Model(&Character{}).Where("group_id=? AND user_id=?", 600, 10).Update("silver", 1000).Error; err != nil {
		t.Fatal(err)
	}
	initial, _, err := s.Profile(ctx, 600, 10)
	if err != nil {
		t.Fatal(err)
	}
	req.MessageID = 2
	req.Action = "draw"
	req.Argument = "5"
	results := make(chan Result, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() { defer wg.Done(); r, err := s.Operate(ctx, req); results <- r; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	fresh := 0
	var receipt Result
	for r := range results {
		if !r.Replayed {
			fresh++
			receipt = r
		}
		rebate := int64(0)
		for _, reward := range r.Rewards {
			rebate += reward.ConvertedSilver
		}
		if len(r.Rewards) != 5 || r.SilverDelta != -300+rebate {
			t.Fatalf("invalid draw result %+v", r)
		}
	}
	if fresh != 1 {
		t.Fatal("duplicate draw settled more than once")
	}
	v, _, err := s.Profile(ctx, 600, 10)
	if err != nil {
		t.Fatal(err)
	}
	p := initial.Character
	expected, err := s.catalog.Draw(&p, receipt.DrawPool, 5, receipt.RewardSeed)
	if err != nil || !reflect.DeepEqual(expected, receipt.Rewards) {
		t.Fatal("draw seed does not reproduce rewards")
	}
	if err := s.catalog.LearnRewards(&p, expected); err != nil {
		t.Fatal(err)
	}
	if v.Character.Silver != p.Silver || !reflect.DeepEqual(v.Character.Inventory, p.Inventory) || v.Character.Draws["jianghu"] != 5 {
		t.Fatal("incorrect assets after replay")
	}
	req.MessageID = 3
	for _, arg := range []string{"9", "5 5", "江湖宝匣 江湖宝匣"} {
		req.Argument = arg
		if _, err := s.Operate(ctx, req); err == nil {
			t.Fatalf("invalid draw arguments accepted: %s", arg)
		}
	}
	req.MessageID = 4
	req.Argument = "5"
	if err := s.database.Model(&Character{}).Where("group_id=? AND user_id=?", 600, 10).Update("silver", 1).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.Operate(ctx, req); err == nil {
		t.Fatal("draw overdraw accepted")
	}
	after, _, err := s.Profile(ctx, 600, 10)
	if err != nil || !reflect.DeepEqual(v.Character.Inventory, after.Character.Inventory) || after.Character.Silver != 1 || after.Character.Draws["jianghu"] != 5 {
		t.Fatal("failed draw changed assets")
	}
	if _, _, err := s.Profile(ctx, 601, 10); err != ErrNotJoined {
		t.Fatal("inventory leaked across groups")
	}
	var count int64
	if err := s.database.Model(&LedgerEntry{}).Where("group_id=? AND reason='draw'", 600).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("draw debit ledger not exactly once")
	}
}

func TestInventoryMigrationPreservesLegacyCharacter(t *testing.T) {
	dsn := pgtest.NewSchema(t)
	cfg := config.DatabaseConfig{DSN: dsn, MaxOpen: 2, MaxIdle: 1, ConnMaxLifetime: 5}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	defer sqlDB.Close()
	if err := database.Exec(`ALTER TABLE wuxia_characters DROP COLUMN inventory, DROP COLUMN equipment, DROP COLUMN learned_skills, DROP COLUMN active_skill, DROP COLUMN draws; DELETE FROM schema_migrations WHERE version=21`).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec(`INSERT INTO wuxia_characters (group_id,user_id,name,profession_id,experience,silver,mastery,weapon_kind,weapons,armor,charm,portrait,created_at,updated_at) VALUES (900,10,'旧侠客','swordsman',360,456,2,'剑','{"剑":"剑-2","刀":"刀-1"}',23,17,'',now(),now())`).Error; err != nil {
		t.Fatal(err)
	}
	upgraded, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	upgradedSQL, _ := upgraded.DB()
	defer upgradedSQL.Close()
	v, _, err := New(upgraded).Profile(context.Background(), 900, 10)
	if err != nil {
		t.Fatal(err)
	}
	p := v.Character
	if p.Inventory["剑-2"] != 1 || p.Inventory["刀-1"] != 1 || p.Silver != 456 || p.Experience != 360 || p.Armor != 23 || p.Charm != 17 || v.Weapon.ID != "剑-2" {
		t.Fatalf("migration lost old assets: %+v", p)
	}
}

func TestInventoryCatalogRejectsAmbiguousReferences(t *testing.T) {
	for _, mutate := range []func(*Catalog){
		func(c *Catalog) { c.Items[0].Name = c.Weapons[0].ID },
		func(c *Catalog) { c.Items[4].SkillID = c.Skills[0].Name },
		func(c *Catalog) { c.Pools[0].Entries[0].Reward.ItemID = "missing" },
	} {
		c := DefaultCatalog()
		mutate(&c)
		raw, _ := json.Marshal(c)
		if _, err := ParseCatalog(raw); err == nil {
			t.Fatal("ambiguous or missing reference accepted")
		}
	}
}
