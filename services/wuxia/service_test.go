package wuxia

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/pgtest"
)

func testService(t *testing.T) *Service {
	t.Helper()
	database, err := db.Open(config.DatabaseConfig{DSN: pgtest.NewSchema(t), MaxOpen: 12, MaxIdle: 2, ConnMaxLifetime: 5})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql, _ := database.DB(); _ = sql.Close() })
	s := New(database)
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	s.now = func() time.Time { return now }
	return s
}

func TestDailyReplayConcurrencyAndIsolation(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	req := Request{BotID: 1, GroupID: 100, UserID: 10, MessageID: 1, Name: "游侠", Action: "daily"}
	const n = 12
	var wg sync.WaitGroup
	errs := make(chan error, n)
	results := make(chan Result, n)
	for range n {
		wg.Add(1)
		go func() { defer wg.Done(); r, err := s.Operate(ctx, req); errs <- err; results <- r }()
	}
	wg.Wait()
	close(errs)
	close(results)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	fresh := 0
	var first Result
	for r := range results {
		if !r.Replayed {
			fresh++
			first = r
		}
		if len(r.Encounters) != 3 || r.Daily.Signed {
			t.Fatal("incomplete daily")
		}
	}
	if fresh != 1 {
		t.Fatalf("fresh settlements=%d", fresh)
	}
	if !first.Joined {
		t.Fatal("first settlement did not record character creation")
	}
	v, day, err := s.Profile(ctx, 100, 10)
	if err != nil || v.Character.Silver != first.Actor.Character.Silver || day.Runs != 3 {
		t.Fatalf("profile=%+v day=%+v err=%v", v, day, err)
	}
	for i := 2; i < 12; i++ {
		req.MessageID = int32(i)
		r, err := s.Operate(ctx, req)
		if err != nil || r.Joined || r.SilverDelta != 0 || r.ExperienceDelta != 0 || len(r.Encounters) != 0 {
			t.Fatalf("daily double-claimed: %+v %v", r, err)
		}
	}
	ledger, err := s.Ledger(ctx, 100, 10)
	if err != nil || len(ledger) != 1 {
		t.Fatalf("ledger=%+v err=%v", ledger, err)
	}
	if _, _, err := s.Profile(ctx, 101, 10); err != ErrNotJoined {
		t.Fatalf("cross-group leak: %v", err)
	}
	req.GroupID = 101
	req.MessageID = 1
	r, err := s.Operate(ctx, req)
	if err != nil || len(r.Encounters) != 3 {
		t.Fatalf("isolated daily %+v %v", r, err)
	}
	old := s.now()
	s.now = func() time.Time { return old.AddDate(0, 0, 1) }
	req.GroupID = 100
	req.MessageID = 20
	r, err = s.Operate(ctx, req)
	if err != nil || len(r.Encounters) != 3 || r.Daily.Runs != 3 {
		t.Fatalf("new day %+v %v", r, err)
	}
}

func TestMixedActionsAndRollback(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	req := Request{BotID: 1, GroupID: 200, UserID: 10, MessageID: 1, Name: "剑客", Action: "adventure"}
	r, err := s.Operate(ctx, req)
	if err != nil || r.Daily.Runs != 3 || r.Daily.Signed {
		t.Fatalf("one adventure %+v %v", r, err)
	}
	req.MessageID++
	req.Action = "daily"
	r, err = s.Operate(ctx, req)
	if err != nil || len(r.Encounters) != 0 || r.Daily.Signed || r.Daily.Runs != 3 {
		t.Fatalf("mixed settlement %+v %v", r, err)
	}
	if err := s.database.Model(&Character{}).Where("group_id=? AND user_id=?", 200, 10).Update("silver", 1000).Error; err != nil {
		t.Fatal(err)
	}
	before := int64(1000)
	req.MessageID++
	req.Action = "profession"
	req.Argument = "狂刀"
	r, err = s.Operate(ctx, req)
	if err != nil || r.Actor.Profession.Name != "狂刀" || r.Actor.Character.Silver != before {
		t.Fatalf("profession %+v %v", r, err)
	}
	req.MessageID++
	req.Action = "train"
	req.Argument = ""
	r, err = s.Operate(ctx, req)
	if err != nil || r.SilverDelta != -50 {
		t.Fatalf("train %+v %v", r, err)
	}
	v, _, _ := s.Profile(ctx, 200, 10)
	if err := s.database.Model(&Character{}).Where("group_id=? AND user_id=?", 200, 10).Update("silver", 0).Error; err != nil {
		t.Fatal(err)
	}
	req.MessageID++
	if _, err = s.Operate(ctx, req); err == nil {
		t.Fatal("insufficient funds accepted")
	}
	after, _, _ := s.Profile(ctx, 200, 10)
	if after.Character.Mastery != v.Character.Mastery || after.Character.Silver != 0 {
		t.Fatal("failed operation partially applied")
	}
}

func TestMirrorDuelDoesNotMutateOpponent(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	req := Request{BotID: 1, GroupID: 300, UserID: 10, MessageID: 1, Name: "甲", Action: "daily"}
	if _, err := s.Operate(ctx, req); err != nil {
		t.Fatal(err)
	}
	req.UserID = 20
	req.MessageID = 2
	req.Name = "乙"
	if _, err := s.Operate(ctx, req); err != nil {
		t.Fatal(err)
	}
	before, _, _ := s.Profile(ctx, 300, 20)
	req.UserID = 10
	req.Name = "甲"
	req.MessageID = 3
	req.Action = "duel"
	req.TargetID = 20
	r, err := s.Operate(ctx, req)
	if err != nil || r.Battle == nil || r.SilverDelta != 100 {
		t.Fatalf("duel %+v %v", r, err)
	}
	after, _, _ := s.Profile(ctx, 300, 20)
	if after.Character.Silver != before.Character.Silver || after.Character.Experience != before.Character.Experience || !after.Character.UpdatedAt.Equal(before.Character.UpdatedAt) {
		t.Fatal("opponent was mutated")
	}
	replayed, err := s.Operate(ctx, req)
	if err != nil || !replayed.Replayed || replayed.Battle.Seed != r.Battle.Seed {
		t.Fatal("duel rerolled")
	}
	originalJSON, _ := json.Marshal(r.Battle)
	replayJSON, _ := json.Marshal(replayed.Battle)
	if string(originalJSON) != string(replayJSON) || r.Battle.Version != DuelVersion || r.Battle.WinProbability[0] < .1 || r.Battle.WinProbability[0] > .9 || r.Battle.Left.Portrait == "" || r.Battle.Right.Portrait == "" {
		t.Fatal("duel snapshot lost probability, score or appearance")
	}
	req.MessageID++
	if _, err := s.Operate(ctx, req); err == nil {
		t.Fatal("duel cooldown bypassed")
	}
}

func TestDuelCreatesNewPlayersWithoutClaimingDaily(t *testing.T) {
	s := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req := Request{BotID: 1, GroupID: 400, UserID: 10, TargetID: 20, MessageID: 1, Name: "甲", Action: "duel"}
	r, err := s.Operate(ctx, req)
	if err != nil || r.Battle == nil || !r.Joined {
		t.Fatalf("new players could not duel: %+v %v", r, err)
	}
	replay, err := s.Operate(ctx, req)
	if err != nil || !replay.Replayed || replay.Battle.Seed != r.Battle.Seed {
		t.Fatalf("replay rerolled: %+v %v", replay, err)
	}
	for _, id := range []int64{10, 20} {
		v, d, err := s.Profile(ctx, 400, id)
		if err != nil || v.Level != 1 || v.Character.Silver != map[int64]int64{10: 100, 20: 0}[id] || v.Character.Experience != map[int64]int64{10: 20, 20: 0}[id] || d.DuelRewarded != (id == 10) || d.Signed || d.Runs != 0 {
			t.Fatalf("duel claimed rewards: %+v %+v %v", v, d, err)
		}
		req.UserID = id
		req.MessageID = int32(id)
		req.Action = "daily"
		daily, err := s.Operate(ctx, req)
		if err != nil || daily.Joined || daily.Daily.Signed || daily.Daily.Runs != 3 {
			t.Fatalf("daily lost after duel: %+v %v", daily, err)
		}
		var encounterSilver int64
		for _, encounter := range daily.Encounters {
			encounterSilver += encounter.Silver
		}
		if daily.SilverDelta-encounterSilver != 0 {
			t.Fatal("unexpected signup reward")
		}
	}
	if _, _, err := s.Profile(ctx, 401, 20); err != ErrNotJoined {
		t.Fatalf("cross-group creation: %v", err)
	}
}

func TestConcurrentNewPlayerDuels(t *testing.T) {
	s := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		go func(i int) {
			<-start
			actor, target, msg := int64(10), int64(20), int32(1)
			if i%2 == 1 {
				actor, target, msg = 20, 10, 2
			}
			_, err := s.Operate(ctx, Request{BotID: 1, GroupID: 500, UserID: actor, TargetID: target, MessageID: msg, Name: "新侠客", Action: "duel"})
			results <- err
		}(i)
	}
	close(start)
	for range 12 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	for table, want := range map[string]int64{"wuxia_characters": 2, "wuxia_operations": 2, "wuxia_ledger": 2} {
		var n int64
		if err := s.database.Table(table).Count(&n).Error; err != nil || n != want {
			t.Fatalf("%s count=%d want=%d: %v", table, n, want, err)
		}
	}
}
