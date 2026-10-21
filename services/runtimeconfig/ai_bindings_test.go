package runtimeconfig

import (
	"testing"

	"github.com/Yuelioi/yueling-go/config"
)

func TestBindingsSnapshotReplacementAndUnrelatedSaves(t *testing.T) {
	for _, tc := range []struct {
		name        string
		replacement any
		want        map[string]string
	}{
		{"empty", map[string]string{}, map[string]string{}},
		{"null", nil, nil},
		{"retained override", map[string]string{"summary": "off"}, map[string]string{"summary": "off"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := fixture(t)
			if _, err := update(m, "ai", "ai.bindings", map[string]string{"conversation": "off", "vision": "off", "memory": "off", "summary": "off"}); err != nil {
				t.Fatal(err)
			}
			if _, err := update(m, "ai", "ai.bindings", tc.replacement); err != nil {
				t.Fatal(err)
			}
			got := config.Current().AI.Bindings
			if len(got) != len(tc.want) {
				t.Fatalf("removed binding survived: got %v want %v", got, tc.want)
			}
			for key, want := range tc.want {
				if got[key] != want {
					t.Fatalf("retained binding changed: %s", key)
				}
			}
			if _, err := update(m, "bot", "bot.name", "新名字"); err != nil {
				t.Fatal(err)
			}
			got = config.Current().AI.Bindings
			if len(got) != len(tc.want) {
				t.Fatal("unrelated save changed bindings", got)
			}
			for key, want := range tc.want {
				if got[key] != want {
					t.Fatal("unrelated save changed retained override")
				}
			}
		})
	}
}
