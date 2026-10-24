package wuxia

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestPlayableJourneySurvivesRestartAndNextDay(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	snapshot := func(v View) string {
		t.Helper()
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if err := s.SetSettings(ctx, 400, Settings{DailyRuns: 3, SignSilver: 200, PracticeBase: 50}); err != nil {
		t.Fatal(err)
	}
	req := Request{BotID: 1, GroupID: 400, UserID: 10, Name: "新入江湖"}
	operate := func(action, argument string) Result {
		t.Helper()
		req.MessageID++
		req.Action, req.Argument = action, argument
		r, err := s.Operate(ctx, req)
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		return r
	}
	first := operate("daily", "")
	firstRequest := req
	if !first.Joined || first.SettledAt.IsZero() || first.Daily.Signed || first.Daily.Runs != 3 || len(first.Encounters) != 3 || first.Actor.Character.Mastery != 0 {
		t.Fatalf("incomplete first journey: %+v", first)
	}
	if err := s.database.Model(&Character{}).Where("group_id=? AND user_id=?", 400, 10).Update("silver", first.Actor.Character.Silver+1000).Error; err != nil {
		t.Fatal(err)
	}
	trained := operate("train", "")
	if trained.Joined || trained.Actor.Character.Mastery != 1 || trained.SilverDelta != -50 || trained.Actor.Character.Silver != first.Actor.Character.Silver+1000-50 {
		t.Fatalf("training did not settle exactly once: %+v", trained)
	}
	switched := operate("profession", "狂刀")
	if switched.Actor.Profession.ID != "blade" || switched.Actor.Character.Weapons[first.Actor.Weapon.Kind] == "" || switched.Actor.Character.Weapons["刀"] == "" || switched.SilverDelta != 0 || switched.Actor.Character.Mastery != 1 {
		t.Fatalf("switch lost growth or equipment: %+v", switched)
	}
	armed := operate("weapon", "刀")
	if armed.Actor.Weapon.Kind != "刀" || armed.Actor.Profession.ID != "blade" {
		t.Fatalf("independent weapon failed: %+v", armed)
	}
	portrait := operate("portrait", "3")
	if portrait.Actor.Fighter().Portrait != "boxer" || portrait.Actor.Stats != armed.Actor.Stats {
		t.Fatal("appearance changed combat stats")
	}
	clock := s.now()
	s = New(s.database)
	s.now = func() time.Time { return clock }
	replay, err := s.Operate(ctx, firstRequest)
	if err != nil || !replay.Replayed || !reflect.DeepEqual(replay.Encounters, first.Encounters) {
		t.Fatalf("restart rerolled a settled journey: %v", err)
	}
	latest, err := s.LatestResult(ctx, 1, 400, 10)
	if err != nil || !latest.Replayed || snapshot(latest.Actor) != snapshot(portrait.Actor) {
		t.Fatalf("latest actor mismatch: err=%v\ngot=%#v\nwant=%#v", err, latest.Actor, portrait.Actor)
	}
	for _, identity := range [][3]int64{{2, 400, 10}, {1, 401, 10}, {1, 400, 20}} {
		if _, err := s.LatestResult(ctx, identity[0], identity[1], identity[2]); err == nil {
			t.Fatalf("report leaked to %v", identity)
		}
	}
	clock = clock.Add(-time.Minute)
	resetPortrait := operate("portrait", "0")
	latest, err = s.LatestResult(ctx, 1, 400, 10)
	if err != nil || snapshot(latest.Actor) != snapshot(resetPortrait.Actor) {
		t.Fatal("latest report depended on wall-clock ordering")
	}
	exhausted := operate("daily", "")
	if exhausted.Joined || len(exhausted.Encounters) != 0 || exhausted.SilverDelta != 0 || exhausted.ExperienceDelta != 0 {
		t.Fatal("daily exhausted but still rewarded")
	}
	clock = clock.AddDate(0, 0, 1)
	nextDay := operate("daily", "")
	if nextDay.Joined || nextDay.Daily.Day == first.Daily.Day || nextDay.Daily.Signed || nextDay.Daily.Runs != 3 || len(nextDay.Encounters) != 3 {
		t.Fatal("next-day journey failed to reset")
	}
	ledger, err := s.Ledger(ctx, 400, 10)
	expectedLedger := 1
	if first.SilverDelta != 0 {
		expectedLedger++
	}
	if nextDay.SilverDelta != 0 {
		expectedLedger++
	}
	if err != nil || len(ledger) != expectedLedger {
		t.Fatalf("unexpected ledger: %+v %v", ledger, err)
	}
	var sum int64
	for _, entry := range ledger {
		sum += entry.Delta
	}
	current, _, err := s.Profile(ctx, 400, 10)
	if err != nil || current.Character.Silver != sum+1000 {
		t.Fatal("balance no longer matches the complete ledger")
	}
}
