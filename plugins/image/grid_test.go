package image

import "testing"

func TestFormatOptionRowsUsesTwoOptionsPerLine(t *testing.T) {
	got := formatOptionRows([]string{"① 火锅", "② 烧烤", "③ 拉面", "④ 炒饭"})
	want := "① 火锅  ② 烧烤\n③ 拉面  ④ 炒饭"
	if got != want {
		t.Fatalf("formatOptionRows() = %q, want %q", got, want)
	}
}

func TestFormatOptionRowsKeepsOddLastOption(t *testing.T) {
	got := formatOptionRows([]string{"① 火锅", "② 烧烤", "③ 拉面"})
	want := "① 火锅  ② 烧烤\n③ 拉面"
	if got != want {
		t.Fatalf("formatOptionRows() = %q, want %q", got, want)
	}
}

func TestDisplayLabel(t *testing.T) {
	cases := []struct {
		name, stem, want string
	}{
		{"name_hash", "麻辣烫_0123456789abcdef", "麻辣烫"},
		{"name with underscore", "麻 辣_烫_0123456789abcdef", "麻 辣_烫"},
		{"legacy pure hash", "0123456789abcdef", "0123456789abcdef"},
		{"plain name no hash", "麻辣烫", "麻辣烫"},
		{"trailing seg not 16", "麻辣烫_abc", "麻辣烫_abc"},
		{"trailing seg not hex", "麻辣烫_zzzzzzzzzzzzzzzz", "麻辣烫_zzzzzzzzzzzzzzzz"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := displayLabel(c.stem); got != c.want {
				t.Fatalf("displayLabel(%q) = %q, want %q", c.stem, got, c.want)
			}
		})
	}
}
