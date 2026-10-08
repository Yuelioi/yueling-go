package db

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestJoinReviewResolution(t *testing.T) {
	global := JoinReviewConfig{Mode: JoinModeOverride, Allow: []string{"交流"}, Deny: []string{"广告"}}
	for _, tc := range []struct {
		name  string
		local JoinReviewConfig
		want  JoinReviewConfig
	}{
		{"inherit", JoinReviewConfig{Mode: JoinModeInherit}, global},
		{"empty override stays empty", JoinReviewConfig{Mode: JoinModeOverride}, JoinReviewConfig{Mode: JoinModeOverride}},
		{"independent", JoinReviewConfig{Mode: JoinModeOverride, Allow: []string{"学习"}}, JoinReviewConfig{Mode: JoinModeOverride, Allow: []string{"学习"}}},
		{"disabled", JoinReviewConfig{Mode: JoinModeDisabled, Allow: []string{"*"}}, JoinReviewConfig{Mode: JoinModeDisabled, Allow: []string{}, Deny: []string{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveJoinReview(tc.local, global); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
	global.Mode = JoinModeDisabled
	if got := resolveJoinReview(JoinReviewConfig{Mode: JoinModeInherit}, global); len(got.Allow) != 0 || len(got.Deny) != 0 {
		t.Fatalf("disabled global applies rules: %+v", got)
	}
}

func TestJoinReviewValidation(t *testing.T) {
	cfg, err := ValidateJoinReview(0, JoinReviewConfig{Mode: JoinModeOverride, Allow: []string{" ABC ", "abc", "", "*"}})
	if err != nil || !reflect.DeepEqual(cfg.Allow, []string{"abc", "*"}) {
		t.Fatalf("normalize: %+v %v", cfg, err)
	}
	for _, tc := range []struct {
		id  int64
		cfg JoinReviewConfig
	}{
		{0, JoinReviewConfig{Mode: JoinModeInherit}}, {-1, JoinReviewConfig{Mode: JoinModeOverride}},
		{123, JoinReviewConfig{Mode: "typo"}}, {123, JoinReviewConfig{Mode: JoinModeOverride, Allow: []string{strings.Repeat("字", 129)}}},
	} {
		if _, err := ValidateJoinReview(tc.id, tc.cfg); err == nil {
			t.Fatalf("accepted invalid config %+v", tc)
		}
	}
}

func TestJoinReviewPersistenceAndCommands(t *testing.T) {
	initPostgresForTest(t)
	mustSet := func(id int64, cfg JoinReviewConfig) {
		t.Helper()
		if err := SetJoinReview(id, cfg); err != nil {
			t.Fatal(err)
		}
	}
	get := func(id int64) JoinReviewState {
		t.Helper()
		s, err := GetJoinReview(id)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	mustSet(0, JoinReviewConfig{Mode: JoinModeOverride, Allow: []string{"全局"}, Deny: []string{"广告"}})
	if s := get(123); s.Config.Mode != JoinModeInherit || !reflect.DeepEqual(s.Effective.Allow, []string{"全局"}) {
		t.Fatalf("inherit: %+v", s)
	}
	mustSet(123, JoinReviewConfig{Mode: JoinModeOverride})
	if s := get(123); len(s.Effective.Allow) != 0 {
		t.Fatalf("empty override: %+v", s)
	}
	mustSet(123, JoinReviewConfig{Mode: JoinModeDisabled, Allow: []string{"*"}, Deny: []string{"广告"}})
	if s := get(123); len(s.Effective.Allow) != 0 || len(s.Config.Allow) != 1 {
		t.Fatalf("disabled: %+v", s)
	}
	if err := SetGroupJoinRules(123, JoinActionAllow, []string{"命令"}); err != nil {
		t.Fatal(err)
	}
	if s := get(123); s.Config.Mode != JoinModeOverride || !reflect.DeepEqual(s.Effective.Allow, []string{"命令"}) || !reflect.DeepEqual(s.Effective.Deny, []string{"广告"}) {
		t.Fatalf("command sync: %+v", s)
	}
	mustSet(123, JoinReviewConfig{Mode: JoinModeInherit})
	mustSet(0, JoinReviewConfig{Mode: JoinModeOverride, Allow: []string{"更新"}})
	if s := get(123); !reflect.DeepEqual(s.Effective.Allow, []string{"更新"}) {
		t.Fatalf("stale config: %+v", s)
	}
	if _, err := AddGroupJoinRule(456, JoinActionAllow, "旧规则"); err != nil {
		t.Fatal(err)
	}
	if s := get(456); s.Config.Mode != JoinModeOverride || !reflect.DeepEqual(s.Effective.Allow, []string{"旧规则"}) {
		t.Fatalf("legacy: %+v", s)
	}
	if err := SetGroupJoinRules(456, JoinActionAllow, nil); err != nil {
		t.Fatal(err)
	}
	if s := get(456); s.Config.Mode != JoinModeOverride || len(s.Effective.Allow) != 0 {
		t.Fatalf("clear inherited unexpectedly: %+v", s)
	}
}

func TestJoinReviewMigrationPreservesExistingGroups(t *testing.T) {
	initPostgresForTest(t)
	if _, err := AddGroupJoinRule(123, JoinActionAllow, "旧规则"); err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec("DROP TABLE group_join_settings").Error; err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("migrations/012_join_review_settings.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	var setting GroupJoinSetting
	if err := DB.Where("group_id = ?", 123).First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if setting.Mode != JoinModeOverride {
		t.Fatalf("migrated mode: %s", setting.Mode)
	}
	if err := SetJoinReview(0, JoinReviewConfig{Mode: JoinModeOverride, Allow: []string{"全局"}}); err != nil {
		t.Fatal(err)
	}
	state, err := GetJoinReview(123)
	if err != nil || !reflect.DeepEqual(state.Effective.Allow, []string{"旧规则"}) {
		t.Fatalf("migration changed behavior: %+v %v", state, err)
	}
}
