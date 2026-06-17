package bot

import (
	"strings"
	"testing"
	"time"
)

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
