// Package tools registers all built-in AI tools via init() functions.
// Import it with a blank identifier to activate all tools:
//
//	import _ "github.com/Yuelioi/yueling-go/ai/tools"
package tools

import (
	"context"
	"io"
	"net/http"
	"time"
)

// httpClient is a shared HTTP client for AI tool network calls.
// Prefer httpclient.Direct for new code in the plugins/ layer.
var httpClient = &http.Client{Timeout: 10 * time.Second}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return httpClient.Do(req)
}
func httpPost(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return httpClient.Do(req)
}
