package codexauth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type credential struct {
	Mode   string `json:"auth_mode"`
	Tokens struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
	} `json:"tokens"`
	LastRefresh json.RawMessage `json:"last_refresh,omitempty"`
}

type Status struct {
	Imported  bool   `json:"imported"`
	AccountID string `json:"account_id,omitempty"`
}

func Directory(dataDir, id string) (string, error) {
	if !validID.MatchString(id) || id == "off" {
		return "", errors.New("连接 ID 无效")
	}
	if dataDir == "" {
		dataDir = "data"
	}
	return filepath.Abs(filepath.Join(dataDir, "codex", id))
}

func Normalize(data []byte) ([]byte, error) {
	var c credential
	if len(data) > 64<<10 || json.Unmarshal(data, &c) != nil || c.Mode != "" && c.Mode != "chatgpt" || c.Tokens.AccessToken == "" || c.Tokens.RefreshToken == "" || c.Tokens.AccountID == "" {
		return nil, errors.New("需要有效的 Codex ChatGPT 登录文件（含可刷新的订阅登录态）")
	}
	c.Mode = "chatgpt"
	return json.Marshal(c)
}

func Import(dataDir, id string, data []byte, replace bool) error {
	normalized, err := Normalize(data)
	if err != nil {
		return err
	}
	dir, err := Directory(dataDir, id)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return errors.New("无法创建订阅目录")
	}
	if err = os.Chmod(dir, 0700); err != nil {
		return errors.New("无法保护订阅目录")
	}
	target := filepath.Join(dir, "auth.json")
	f, err := os.CreateTemp(dir, ".auth-")
	if err != nil {
		return errors.New("无法保存登录态")
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(normalized); err != nil {
		f.Close()
		return errors.New("无法保存登录态")
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return errors.New("无法保存登录态")
	}
	if err = f.Close(); err != nil {
		return errors.New("无法保存登录态")
	}
	if replace {
		err = os.Rename(f.Name(), target)
	} else {
		err = os.Link(f.Name(), target)
	}
	if err != nil {
		return errors.New("无法导入；若已有登录态，请明确选择替换")
	}
	return nil
}

func ReadStatus(dataDir, id string) Status {
	dir, err := Directory(dataDir, id)
	if err != nil {
		return Status{}
	}
	file, err := os.ReadFile(filepath.Join(dir, "auth.json"))
	if err != nil {
		return Status{}
	}
	normalized, err := Normalize(file)
	if err != nil {
		return Status{}
	}
	var c credential
	_ = json.Unmarshal(normalized, &c)
	return Status{Imported: true, AccountID: c.Tokens.AccountID}
}
