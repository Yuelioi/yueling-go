package feed

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Yuelioi/yueling-go/db"
)

func translatePendingItems(ctx context.Context, items []db.FeedPendingItem, translator Translator) ([]db.FeedPendingItem, error) {
	return translatePendingItemsMatching(ctx, items, func(db.FeedPendingItem) bool { return true }, translator)
}

func translatePendingItemsMatching(ctx context.Context, items []db.FeedPendingItem, shouldTranslate func(db.FeedPendingItem) bool, translator Translator) ([]db.FeedPendingItem, error) {
	if len(items) == 0 {
		return nil, nil
	}
	if translator == nil {
		return nil, fmt.Errorf("翻译服务未配置")
	}

	translated := append([]db.FeedPendingItem(nil), items...)
	translateCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	semaphore := make(chan struct{}, maxConcurrentTranslations)
	errorCh := make(chan error, 1)
	var wait sync.WaitGroup
	for index := range translated {
		index := index
		if !shouldTranslate(translated[index]) || strings.TrimSpace(translated[index].Title) == "" {
			continue
		}
		wait.Add(1)
		go func() {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-translateCtx.Done():
				return
			}
			text, err := translator(translateCtx, translated[index].Title)
			if err == nil {
				text = cleanFeedText(text, MaxItemMaxChars)
				if text == "" {
					err = fmt.Errorf("翻译服务返回空译文")
				}
			}
			if err != nil {
				select {
				case errorCh <- err:
					cancel()
				default:
				}
				return
			}
			translated[index].Title = text
		}()
	}
	wait.Wait()
	select {
	case err := <-errorCh:
		return nil, err
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return translated, nil
}

func formatPendingNotification(items []db.FeedPendingItem, itemMaxChars int) string {
	var builder strings.Builder
	builder.WriteString("📡 订阅更新")
	lastFeed := ""
	for _, item := range items {
		if item.FeedName != lastFeed {
			builder.WriteString("\n\n【")
			builder.WriteString(item.FeedName)
			builder.WriteString("】")
			lastFeed = item.FeedName
		}
		builder.WriteString("\n\n• ")
		title := item.Title
		if itemMaxChars > 0 {
			title = cleanFeedText(title, itemMaxChars)
		}
		builder.WriteString(title)
		if item.Link != "" {
			builder.WriteByte('\n')
			builder.WriteString(item.Link)
		}
	}
	return builder.String()
}
