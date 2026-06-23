package image

import "testing"

func TestExtractImageURL(t *testing.T) {
	cases := []struct {
		name, body, path, want string
		wantErr                bool
	}{
		{"object field", `{"data":{"url":"x"}}`, "data.url", "x", false},
		{"list of strings random", `{"data":["only"]}`, "data", "only", false},
		{"list of objects random", `{"data":[{"url":"a"}]}`, "data.url", "a", false},
		{"nested", `{"a":{"b":{"c":"deep"}}}`, "a.b.c", "deep", false},
		{"missing key", `{"data":{}}`, "data.url", "", true},
		{"not a string", `{"data":{"url":123}}`, "data.url", "", true},
		{"bad json", `not json`, "data", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractImageURL([]byte(c.body), c.path)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
