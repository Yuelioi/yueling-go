package meme

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var (
	serverURL string
	client    = &http.Client{Timeout: 30 * time.Second}
	infosMap  map[string]*MemeInfo
)

// MemeInfo describes the requirements for one meme template.
type MemeInfo struct {
	Key      string   `json:"key"`
	Keywords []string `json:"keywords"`
	Params   struct {
		MinImages  int      `json:"min_images"`
		MaxImages  int      `json:"max_images"`
		MinTexts   int      `json:"min_texts"`
		MaxTexts   int      `json:"max_texts"`
		ImageNames []string `json:"image_names"`
	} `json:"params"`
}

// Init connects to the meme server and caches all meme metadata.
func Init(url string) error {
	if url == "" {
		return fmt.Errorf("meme_server not configured")
	}
	serverURL = url

	resp, err := client.Get(url + "/meme/infos")
	if err != nil {
		return fmt.Errorf("meme server unreachable: %w", err)
	}
	defer resp.Body.Close()

	var infos []MemeInfo
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return fmt.Errorf("parse meme infos: %w", err)
	}

	infosMap = make(map[string]*MemeInfo, len(infos))
	for i := range infos {
		infosMap[infos[i].Key] = &infos[i]
	}
	return nil
}

// AllKeys returns all registered meme keys.
func AllKeys() []string {
	keys := make([]string, 0, len(infosMap))
	for k := range infosMap {
		keys = append(keys, k)
	}
	return keys
}

// GetInfo returns metadata for a meme key, nil if unknown.
func GetInfo(key string) *MemeInfo {
	return infosMap[key]
}

// Generate uploads images to the server, generates the meme, and returns the result bytes + content-type.
func Generate(key string, images [][]byte, texts []string, options map[string]any) ([]byte, string, error) {
	// Upload images and collect temporary IDs.
	imageSlots := make([]map[string]string, 0, len(images))
	info := infosMap[key]
	for i, img := range images {
		id, err := uploadImage(img)
		if err != nil {
			return nil, "", fmt.Errorf("upload image %d: %w", i, err)
		}
		name := fmt.Sprintf("arg%d", i)
		if info != nil && i < len(info.Params.ImageNames) {
			name = info.Params.ImageNames[i]
		}
		imageSlots = append(imageSlots, map[string]string{"name": name, "id": id})
	}

	if options == nil {
		options = map[string]any{}
	}

	body, _ := json.Marshal(map[string]any{
		"images":  imageSlots,
		"texts":   texts,
		"options": options,
	})

	req, _ := http.NewRequest("POST", serverURL+"/memes/"+key, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	var genResp struct {
		ImageID string `json:"image_id"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, "", err
	}
	if genResp.ImageID == "" {
		return nil, "", fmt.Errorf("[%s] %s", genResp.Code, genResp.Message)
	}

	return fetchImage(genResp.ImageID)
}

func uploadImage(data []byte) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"base64": base64.StdEncoding.EncodeToString(data),
	})
	req, _ := http.NewRequest("POST", serverURL+"/image/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
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

func fetchImage(id string) ([]byte, string, error) {
	resp, err := client.Get(serverURL + "/image/" + id)
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

// FetchURL downloads bytes from any URL (used to fetch QQ avatars).
func FetchURL(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// QQAvatarURL returns the standard QQ avatar URL for a user ID.
func QQAvatarURL(userID int64) string {
	return fmt.Sprintf("https://q2.qpic.cn/g?b=qq&nk=%d&s=640", userID)
}
