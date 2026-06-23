package image

import "testing"

func TestResolveURL(t *testing.T) {
	cases := []struct {
		name, base, u, want string
	}{
		{"relative + base", "https://pln.yuelili.com", "/api/v1/files/x.png", "https://pln.yuelili.com/api/v1/files/x.png"},
		{"base trailing slash", "https://pln.yuelili.com/", "/api/v1/files/x.png", "https://pln.yuelili.com/api/v1/files/x.png"},
		{"already absolute https", "https://pln.yuelili.com", "https://cdn.other.com/y.png", "https://cdn.other.com/y.png"},
		{"already absolute http", "https://x", "http://cdn/y.png", "http://cdn/y.png"},
		{"relative no base", "", "/api/v1/files/x.png", "/api/v1/files/x.png"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveURL(c.base, c.u); got != c.want {
				t.Fatalf("resolveURL(%q,%q) = %q, want %q", c.base, c.u, got, c.want)
			}
		})
	}
}

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
