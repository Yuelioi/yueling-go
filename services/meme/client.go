package meme

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/services/httpclient"
)

var (
	mu         sync.RWMutex
	serverURL  string
	mc         = &httpclient.Client{Client: &http.Client{Timeout: 30 * time.Second}}
	keyMap     map[string]*MemeInfo
	keywordMap map[string]*MemeInfo
)

func randIntn(n int) int { return rand.Intn(n) }

// MemeInfo mirrors the /meme/infos response.
type MemeInfo struct {
	Key      string   `json:"key"`
	Keywords []string `json:"keywords"`
	Params   struct {
		MinImages    int      `json:"min_images"`
		MaxImages    int      `json:"max_images"`
		MinTexts     int      `json:"min_texts"`
		MaxTexts     int      `json:"max_texts"`
		DefaultTexts []string `json:"default_texts"`
	} `json:"params"`
}

// Init connects to the meme server and caches all meme metadata.
func Init(url string) error {
	if url == "" {
		return fmt.Errorf("meme_server not configured")
	}

	var infos []MemeInfo
	if err := mc.GetJSON(url+"/meme/infos", &infos); err != nil {
		return fmt.Errorf("meme server: %w", err)
	}

	km := make(map[string]*MemeInfo, len(infos))
	kwm := make(map[string]*MemeInfo)
	for i := range infos {
		info := &infos[i]
		km[info.Key] = info
		for _, kw := range info.Keywords {
			kwm[kw] = info
		}
	}

	mu.Lock()
	serverURL = url
	keyMap = km
	keywordMap = kwm
	mu.Unlock()
	return nil
}

// RandomEligible picks a random meme that requires at least 1 image and no text.
func RandomEligible() *MemeInfo {
	mu.RLock()
	defer mu.RUnlock()
	var candidates []*MemeInfo
	for _, info := range keyMap {
		if info.Params.MinTexts > 0 || info.Params.MinImages < 1 {
			continue
		}
		candidates = append(candidates, info)
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates[randIntn(len(candidates))]
}

// TextOnlyKeys returns keys of memes that require no images.
func TextOnlyKeys() []string {
	mu.RLock()
	defer mu.RUnlock()
	var keys []string
	for key, info := range keyMap {
		if info.Params.MinImages == 0 {
			keys = append(keys, key)
		}
	}
	return keys
}

// AllKeywords returns every user-facing keyword across all memes.
func AllKeywords() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(keywordMap))
	for kw := range keywordMap {
		out = append(out, kw)
	}
	return out
}

// GetInfoByKeyword returns the MemeInfo for a keyword, nil if unknown.
func GetInfoByKeyword(kw string) *MemeInfo {
	mu.RLock()
	defer mu.RUnlock()
	return keywordMap[kw]
}

// Generate uploads images, calls /memes/{key}, fetches and returns the result bytes.
func Generate(key string, images [][]byte, texts []string, options map[string]any) ([]byte, string, error) {
	mu.RLock()
	url := serverURL
	mu.RUnlock()

	type imageSlot struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	slots := make([]imageSlot, 0, len(images))
	for i, img := range images {
		id, err := uploadImage(url, img)
		if err != nil {
			return nil, "", fmt.Errorf("upload image %d: %w", i, err)
		}
		slots = append(slots, imageSlot{Name: fmt.Sprintf("arg%d", i), ID: id})
	}

	if texts == nil {
		texts = []string{}
	}
	if options == nil {
		options = map[string]any{}
	}

	body, err := json.Marshal(map[string]any{
		"images":  slots,
		"texts":   texts,
		"options": options,
	})
	if err != nil {
		return nil, "", err
	}

	resp, err := mc.Do(newJSONReq("POST", url+"/memes/"+key, body))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		var errResp struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(rawBody, &errResp) == nil && errResp.Message != "" {
			return nil, "", fmt.Errorf("%s", errResp.Message)
		}
		return nil, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(rawBody))
	}
	var genResp struct {
		ImageID string `json:"image_id"`
	}
	if err := json.Unmarshal(rawBody, &genResp); err != nil || genResp.ImageID == "" {
		return nil, "", fmt.Errorf("unexpected response: %s", string(rawBody))
	}
	return fetchResultImage(url, genResp.ImageID)
}

// GetPreview fetches the preview image for a meme key.
func GetPreview(key string) ([]byte, error) {
	mu.RLock()
	url := serverURL
	mu.RUnlock()

	resp, err := mc.Do(newReq("GET", url+"/memes/"+key+"/preview"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(rawBody))
	}
	var result struct {
		ImageID string `json:"image_id"`
	}
	if err := json.Unmarshal(rawBody, &result); err != nil || result.ImageID == "" {
		return nil, fmt.Errorf("unexpected response: %s", string(rawBody))
	}
	data, _, err := fetchResultImage(url, result.ImageID)
	return data, err
}

// RenderList calls /tools/render_list and returns the rendered image bytes.
func RenderList(excludeKeys []string) ([]byte, error) {
	mu.RLock()
	url := serverURL
	mu.RUnlock()

	if excludeKeys == nil {
		excludeKeys = []string{}
	}
	body, _ := json.Marshal(map[string]any{
		"exclude_memes": excludeKeys,
		"sort_by":       "keywords_pinyin",
	})

	resp, err := mc.Do(newJSONReq("POST", url+"/tools/render_list", body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	var result struct {
		ImageID string `json:"image_id"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %s", string(rawBody))
	}
	if result.ImageID == "" {
		return nil, fmt.Errorf("[%s] %s", result.Code, result.Message)
	}
	data, _, err := fetchResultImage(url, result.ImageID)
	return data, err
}

// FetchURL downloads bytes from an arbitrary URL (for QQ avatars etc.).
func FetchURL(rawURL string) ([]byte, error) {
	return httpclient.Direct.GetBytes(rawURL)
}

// QQAvatarURL returns the QQ avatar URL for a user ID.
func QQAvatarURL(userID int64) string {
	return fmt.Sprintf("https://q.qlogo.cn/headimg_dl?dst_uin=%d&spec=640&img_type=jpg", userID)
}

func uploadImage(base string, data []byte) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"type": "data",
		"data": encodeBase64(data),
	})
	resp, err := mc.Do(newJSONReq("POST", base+"/image/upload", body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ImageID string `json:"image_id"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.ImageID == "" {
		return "", fmt.Errorf("[%s] %s", result.Code, result.Message)
	}
	return result.ImageID, nil
}

func fetchResultImage(base, id string) ([]byte, string, error) {
	resp, err := mc.Do(newReq("GET", base+"/image/"+id))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	return data, ct, nil
}

func newReq(method, url string) *http.Request {
	req, _ := http.NewRequest(method, url, nil)
	return req
}

func newJSONReq(method, url string, body []byte) *http.Request {
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
