package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type deepSeekTransport struct{ base http.RoundTripper }

func (t deepSeekTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.EqualFold(req.URL.Hostname(), "api.deepseek.com") || req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/chat/completions") || req.Body == nil {
		return t.base.RoundTrip(req)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		req.Body.Close()
		return nil, err
	}
	req.Body.Close()
	var payload map[string]json.RawMessage
	if json.Unmarshal(body, &payload) == nil {
		var model, effort string
		_ = json.Unmarshal(payload["model"], &model)
		_ = json.Unmarshal(payload["reasoning_effort"], &effort)
		if strings.HasPrefix(model, "deepseek-v4") || model == "deepseek-flash" {
			switch effort {
			case "none":
				payload["thinking"] = json.RawMessage(`{"type":"disabled"}`)
				delete(payload, "reasoning_effort")
			case "":
			default:
				payload["thinking"] = json.RawMessage(`{"type":"enabled"}`)
			}
			body, err = json.Marshal(payload)
			if err != nil {
				return nil, err
			}
		}
	}
	// Only the native endpoint receives vendor fields; compatible APIs keep their contract.
	cloned := req.Clone(req.Context())
	cloned.Body = io.NopCloser(bytes.NewReader(body))
	cloned.ContentLength = int64(len(body))
	cloned.Header.Set("Content-Length", strconv.Itoa(len(body)))
	cloned.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return t.base.RoundTrip(cloned)
}
