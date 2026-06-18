package bot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// callT 必须遵守传入的响应超时：响应在超时内到达则成功，否则返回 response timeout。
// 守护「大文件上传用 uploadCallTimeout 放宽」的修复——若有人把 upload 路径改回默认 10s
// 或让 callT 忽略超时参数，此测试会失败。见 incidents/2026-06-18-pack-upload-stream-1009。
func TestCallTHonorsTimeout(t *testing.T) {
	if uploadCallTimeout <= defaultCallTimeout {
		t.Fatalf("uploadCallTimeout(%v) 必须大于 defaultCallTimeout(%v)", uploadCallTimeout, defaultCallTimeout)
	}

	// 无人响应 + 短超时 → response timeout（sendCh 有缓冲，确保走到响应等待分支）。
	a := &BotAPI{sendCh: make(chan []byte, 1), done: make(chan struct{})}
	if _, err := a.callT("slow_action", nil, 30*time.Millisecond); err == nil ||
		!strings.Contains(err.Error(), "response timeout") {
		t.Fatalf("want response timeout, got %v", err)
	}

	// 响应在超时内到达 → 成功。deliver 用 call 写入 echo 时存下的 ch。
	sendCh := make(chan []byte, 1)
	b := &BotAPI{sendCh: sendCh, done: make(chan struct{})}
	got := make(chan error, 1)
	go func() { _, err := b.callT("ok_action", nil, time.Second); got <- err }()

	// 等 payload 入队后解析出 echo，再投递响应。
	payload := <-sendCh
	var p struct {
		Echo string `json:"echo"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		t.Fatalf("payload 解析失败: %v", err)
	}
	b.deliver(p.Echo, json.RawMessage(`{}`))

	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("响应已在超时内到达，want nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callT 在收到响应后未返回")
	}
}

// 连接已断（done 关闭）时，call 必须快速返回错误而不是 panic。
// 旧实现里 recvLoop `close(sendCh)`，在途 handler 再 call 会「send on closed channel」panic。
func TestCallOnClosedConnNoPanic(t *testing.T) {
	done := make(chan struct{})
	close(done)
	// sendCh 无缓冲且无人读：迫使 call 走 <-done 分支，确定性命中。
	a := &BotAPI{sendCh: make(chan []byte), done: done}

	got := make(chan error, 1)
	go func() { _, err := a.call("test_action", nil); got <- err }()

	select {
	case err := <-got:
		if err == nil || !strings.Contains(err.Error(), "connection closed") {
			t.Fatalf("want connection closed error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("call 在连接关闭时未及时返回（疑似阻塞/panic 吞掉）")
	}
}
