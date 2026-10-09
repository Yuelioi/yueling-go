package feed

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/db"
)

const (
	MaxSubscriptionsPerGroup  = 10
	initialPollDelay          = 30 * time.Second
	maxItemsPerNotification   = 5
	maxPendingPerDelivery     = 12
	maxConcurrentFetches      = 4
	maxConcurrentTranslations = 3
	maxFailureBackoff         = 6 * time.Hour
	translationTimeout        = 45 * time.Second
)

var (
	ErrSubscriptionLimit     = errors.New("本群订阅数量已达上限")
	ErrDuplicateSubscription = errors.New("本群已经订阅过这个地址")
)

type Sender interface {
	SendGroupText(groupID int64, text string) error
}

type Fetcher func(rawURL string) (*Feed, error)

type Translator func(ctx context.Context, text string) (string, error)

type CheckResult struct {
	Checked   int `json:"checked"`
	Updated   int `json:"updated"`
	Items     int `json:"items"`
	Delivered int `json:"delivered"`
	Queued    int `json:"queued"`
	Failed    int `json:"failed"`
}

type Manager struct {
	store        *db.FeedStore
	fetch        Fetcher
	fetchContext func(context.Context, string) (*Feed, error)
	translate    Translator
	now          func() time.Time

	startMu sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	stopped bool
	runMu   sync.Mutex
}

func NewManager(store *db.FeedStore, fetcher Fetcher) *Manager {
	return NewManagerWithTranslator(store, fetcher, translateToChinese)
}

func NewManagerWithTranslator(store *db.FeedStore, fetcher Fetcher, translator Translator) *Manager {
	var contextual func(context.Context, string) (*Feed, error)
	if fetcher == nil {
		fetcher = Fetch
		contextual = FetchContext
	} else {
		contextual = func(_ context.Context, url string) (*Feed, error) { return fetcher(url) }
	}
	return &Manager{store: store, fetch: fetcher, fetchContext: contextual, translate: translator, now: time.Now}
}
