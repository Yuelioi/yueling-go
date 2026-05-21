// Package tools registers all built-in AI tools via init() functions.
// Import it with a blank identifier to activate all tools:
//
//	import _ "github.com/Yuelioi/yueling-go/ai/tools"
package tools

import (
	"net/http"
	"time"
)

// httpClient is a shared HTTP client for AI tool network calls.
// Prefer httpclient.Direct for new code in the plugins/ layer.
var httpClient = &http.Client{Timeout: 10 * time.Second}
