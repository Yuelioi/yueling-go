package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yuelioi/yueling-go/services/codexauth"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	id := flag.String("connection", "", "后台连接 ID")
	dataDir := flag.String("data-dir", "data", "目标 Bot 数据目录；SSH 模式需填写服务器绝对路径")
	source := flag.String("source", "", "本机 auth.json 路径，- 表示标准输入")
	host := flag.String("ssh", "", "SSH 目标，例如 user@server")
	replace := flag.Bool("replace", false, "替换已有登录态（请先停止使用该连接的请求）")
	flag.Parse()
	dir, err := codexauth.Directory(*dataDir, *id)
	if err != nil {
		return err
	}
	if *source == "" {
		home := os.Getenv("CODEX_HOME")
		if home == "" {
			userHome, _ := os.UserHomeDir()
			home = filepath.Join(userHome, ".codex")
		}
		*source = filepath.Join(home, "auth.json")
	}
	var reader io.Reader = os.Stdin
	if *source != "-" {
		f, err := os.Open(*source)
		if err != nil {
			return fmt.Errorf("未找到登录文件；如果凭据在系统钥匙串，请使用独立 CODEX_HOME 完成文件模式登录后指定 -source")
		}
		defer f.Close()
		reader = f
	}
	data, err := io.ReadAll(io.LimitReader(reader, (64<<10)+1))
	if err != nil {
		return fmt.Errorf("无法读取登录文件")
	}
	data, err = codexauth.Normalize(data)
	if err != nil {
		return err
	}
	if *host == "" {
		if err = codexauth.Import(*dataDir, *id, data, *replace); err != nil {
			return err
		}
	} else {
		if strings.HasPrefix(*host, "-") || strings.ContainsAny(*host, "\r\n") || !filepath.IsAbs(*dataDir) {
			return fmt.Errorf("SSH 目标无效或服务器数据目录不是绝对路径")
		}
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
		target := filepath.Join(dir, "auth.json")
		publish := "ln -- \"$auth_tmp\" " + quote(target)
		if *replace {
			publish = "mv -f -- \"$auth_tmp\" " + quote(target)
		}
		script := "set -eu; umask 077; mkdir -p -- " + quote(dir) + "; chmod 700 -- " + quote(dir) + "; auth_tmp=$(mktemp " + quote(filepath.Join(dir, ".auth-XXXXXX")) + "); trap 'rm -f -- \"$auth_tmp\"' EXIT; cat > \"$auth_tmp\"; " + publish
		cmd := exec.Command("ssh", "--", *host, script)
		cmd.Stdin = bytes.NewReader(data)
		cmd.Stderr = os.Stderr
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("SSH 导入失败；检查目录权限和是否已有登录态")
		}
	}
	fmt.Println("订阅登录态已导入；请在后台测试对应模型，后续由服务器刷新。")
	return nil
}
