package feed

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/db"
)

type HealthResult struct {
	ID        uint   `json:"id"`
	GroupID   int64  `json:"group_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	CheckedAt int64  `json:"checked_at"`
}

func (m *Manager) Diagnose(ctx context.Context, groupID int64, id uint) ([]HealthResult, error) {
	rows, err := m.store.ListAllFeedSubscriptions()
	if err != nil {
		return nil, err
	}
	selected := make([]db.FeedSubscription, 0, len(rows))
	for _, row := range rows {
		if (groupID == 0 || row.GroupID == groupID) && (id == 0 || row.ID == id) {
			selected = append(selected, row)
		}
	}
	return m.diagnoseRows(ctx, selected), nil
}

func (m *Manager) diagnoseRows(ctx context.Context, rows []db.FeedSubscription) []HealthResult {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	type outcome struct{ status, message string }
	urls := make(chan string)
	outcomes := map[string]outcome{}
	var mu sync.Mutex
	var workers sync.WaitGroup
	for i := 0; i < maxConcurrentFetches; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for rawURL := range urls {
				var parsed *Feed
				err := ctx.Err()
				if err == nil {
					parsed, err = m.fetchContext(ctx, rawURL)
				}
				if err == nil && parsed == nil {
					err = errors.New("empty response")
				}
				status, message := healthOutcome(err)
				mu.Lock()
				outcomes[rawURL] = outcome{status, message}
				mu.Unlock()
			}
		}()
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if !seen[row.URL] {
			seen[row.URL] = true
			urls <- row.URL
		}
	}
	close(urls)
	workers.Wait()
	result := make([]HealthResult, 0, len(rows))
	for _, row := range rows {
		outcome := outcomes[row.URL]
		result = append(result, HealthResult{ID: row.ID, GroupID: row.GroupID, Status: outcome.status, Message: outcome.message, CheckedAt: m.now().Unix()})
	}
	return result
}

func healthOutcome(err error) (string, string) {
	if err == nil {
		return "readable", "订阅源可读取；RSSHub 可能返回缓存，不能据此确认上游 token 未过期。"
	}
	text := strings.ToLower(err.Error())
	var networkError net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled), errors.As(err, &networkError) && networkError.Timeout(), strings.Contains(text, "超时"):
		return "timeout", "请求超时或已取消，请检查网络与 RSSHub 服务后重试。"
	case strings.Contains(text, "not configured"):
		return "not_configured", "上游服务缺少配置，请在 RSSHub 中配置对应平台凭证后重试。"
	case strings.Contains(text, "invalid or expired token"), strings.Contains(text, "unauthorized"), strings.Contains(text, "http 401"), strings.Contains(text, "invalid token"), strings.Contains(text, "token expired"), strings.Contains(text, "expired token"), strings.Contains(text, "could not authenticate"), strings.Contains(text, "authentication credentials"), strings.Contains(text, "auth_token"), strings.Contains(text, "login required"):
		return "authentication", "疑似登录凭证失效或缺失，请检查 RSSHub 对应平台的 token / Cookie，更新后重试。"
	case strings.Contains(text, "http 403"):
		return "forbidden", "访问被拒绝（403），可能是凭证、权限或平台风控，不能直接判定 token 过期。"
	case strings.Contains(text, "http 429"):
		return "rate_limited", "请求被限流（429），请稍后重试。"
	case strings.Contains(text, "http 404"):
		return "not_found", "订阅路由或目标不存在（404），请检查地址与账号。"
	default:
		return "unavailable", "订阅暂不可读取，可能是上游服务、网络或内容格式异常；请检查 RSSHub 日志，不能直接判定 token 过期。"
	}
}
