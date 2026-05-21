package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/Yuelioi/yueling-go/config"
)

// Client wraps http.Client with convenience methods for common request patterns.
type Client struct {
	*http.Client
}

var (
	Direct = &Client{&http.Client{Timeout: 10 * time.Second}}
	Proxy  *Client
)

func init() {
	Proxy = Direct
}

func InitProxy() {
	addr := config.C.Tools.Proxy
	if addr == "" {
		return
	}
	u, err := url.Parse(addr)
	if err != nil {
		log.Printf("[httpclient] invalid proxy address %q: %v", addr, err)
		return
	}
	Proxy = &Client{&http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(u)},
		Timeout:   15 * time.Second,
	}}
}

// GetBytes fetches a URL and returns raw body bytes.
// Optional headers in key-value pairs: GetBytes(url, "Accept", "application/json")
// Always sets a default User-Agent; callers may override by passing their own.
func (c *Client) GetBytes(url string, headers ...string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// GetJSON fetches a URL and JSON-decodes the response body into out.
func (c *Client) GetJSON(url string, out any, headers ...string) error {
	data, err := c.GetBytes(url, headers...)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// PostJSON marshals body as JSON, POSTs it, and decodes the response into out (nil to discard).
func (c *Client) PostJSON(url string, body, out any, headers ...string) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Post sends raw bytes with a specified content type and decodes JSON response into out (nil to discard).
func (c *Client) Post(url, contentType string, body []byte, out any) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
