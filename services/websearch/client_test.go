package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSearchAndFetchContract(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("wrong method or credential")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if _, ok := body["model"]; ok {
			t.Error("raw search should not invoke a chat model")
		}
		switch r.URL.Path {
		case "/proxy/v1/tools/search":
			if body["query"] != strings.Repeat("图", 128) || body["count"] != float64(5) || body["need_content"] != false || len([]rune(body["ori_question"].(string))) != 128 {
				t.Errorf("invalid request: %+v", body)
			}
			w.Write([]byte(`{"results":[{"title":"官方资料","url":"https://example.com/a","summary":"给模型的长摘要","snippet":"短摘要","date":null,"authority":-1},{"title":"unsafe","url":"file:///etc/passwd"}]}`))
		case "/proxy/v1/tools/fetch":
			if body["url"] != "https://example.com/a" {
				t.Error(body)
			}
			w.Write([]byte(`{"title":"原文","content":"# 网页正文"}`))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	for _, path := range []string{"/proxy", "/proxy/v1", "/proxy/v1/tools/"} {
		c, err := New(server.URL+path, "fixture-key", time.Second)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := c.Search(context.Background(), strings.Repeat("图", 128), 0, strings.Repeat("问", 140))
		if err != nil || len(rows) != 1 || rows[0].Summary != "给模型的长摘要" || rows[0].Date != nil {
			t.Fatalf("%+v %v", rows, err)
		}
		page, err := c.Fetch(context.Background(), rows[0].URL)
		if err != nil || page.Content != "# 网页正文" || page.URL != rows[0].URL {
			t.Fatalf("%+v %v", page, err)
		}
	}
	if calls != 6 {
		t.Fatal(calls)
	}
}

func TestSearchErrorsAreNotEmptyResultsAndNeverLeakSecrets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		failed bool
	}{
		{"empty", 200, `{"results":[]}`, false},
		{"unauthorized", 401, `{"error":{"message":"private-key"}}`, true},
		{"balance", 402, `{"error":{"message":"private-key"}}`, true},
		{"not_found", 404, `{"error":{"message":"private-key"}}`, true},
		{"malformed", 200, `{"oops":[]}`, true},
		{"invalid_json", 200, `nope private-key`, true},
		{"oversized", 200, strings.Repeat("x", maxResponseBytes+1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			c, _ := New(server.URL, "private-key", time.Second)
			rows, err := c.Search(context.Background(), "资料", 5, "")
			if (err != nil) != tc.failed {
				t.Fatalf("%+v %v", rows, err)
			}
			if err != nil && strings.Contains(err.Error(), "private-key") {
				t.Fatal("credential leaked")
			}
		})
	}
}

func TestSearchValidationCancellationRetryAndRedirect(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(429)
			return
		}
		w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()
	c, _ := New(server.URL, "fixture", time.Second)
	for _, query := range []string{"", strings.Repeat("😀", 129)} {
		if _, err := c.Search(context.Background(), query, 5, ""); err == nil {
			t.Fatal("invalid query accepted")
		}
	}
	if _, err := c.Search(context.Background(), "valid", 11, ""); err == nil {
		t.Fatal("invalid count accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Search(ctx, "valid", 5, ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("canceled or invalid request reached server")
	}
	if _, err := c.Search(context.Background(), "valid", 5, ""); err != nil || calls.Load() != 2 {
		t.Fatalf("retry=%d %v", calls.Load(), err)
	}
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	c, _ = New(redirect.URL, "fixture", time.Second)
	if _, err := c.Search(context.Background(), "valid", 5, ""); err == nil || redirected.Load() != 0 {
		t.Fatal("followed credential redirect")
	}
	for _, raw := range []string{"file:///etc/passwd", "http://127.0.0.1/a", "http://169.254.169.254/a", "http://[::1]/a", "https://user:secret@example.com/a", "http://host.internal/a"} {
		if _, err := c.Fetch(context.Background(), raw); err == nil {
			t.Fatal("unsafe URL accepted", raw)
		}
	}
}
