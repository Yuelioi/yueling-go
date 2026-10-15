package app

import (
	"context"
	"fmt"
	"github.com/Yuelioi/yueling-go/services/imageassets"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Yuelioi/yueling-go/ai"
	aitools "github.com/Yuelioi/yueling-go/ai/tools"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/ai_dispatch"
	"github.com/Yuelioi/yueling-go/plugins/ai_proactive"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme"
	"github.com/Yuelioi/yueling-go/plugins/bilibili"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/plugins/emoticon"
	"github.com/Yuelioi/yueling-go/plugins/funny"
	"github.com/Yuelioi/yueling-go/plugins/game"
	"github.com/Yuelioi/yueling-go/plugins/group"
	"github.com/Yuelioi/yueling-go/plugins/image"
	"github.com/Yuelioi/yueling-go/plugins/imageops"
	groupknowledge "github.com/Yuelioi/yueling-go/plugins/knowledge"
	"github.com/Yuelioi/yueling-go/plugins/memo"
	"github.com/Yuelioi/yueling-go/plugins/quotation"
	"github.com/Yuelioi/yueling-go/plugins/random"
	"github.com/Yuelioi/yueling-go/plugins/subscription"
	"github.com/Yuelioi/yueling-go/plugins/system"
	"github.com/Yuelioi/yueling-go/plugins/tools"
	"github.com/Yuelioi/yueling-go/scheduler"
	"github.com/Yuelioi/yueling-go/services"
	bservice "github.com/Yuelioi/yueling-go/services/bilibili"
	"github.com/Yuelioi/yueling-go/services/feed"
	"github.com/Yuelioi/yueling-go/services/httpapi"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/knowledge"
	"github.com/Yuelioi/yueling-go/services/llm"
	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/Yuelioi/yueling-go/services/meme"
	"github.com/Yuelioi/yueling-go/services/runtimeconfig"
	"github.com/Yuelioi/yueling-go/services/webui"
)

const lockPort = "127.0.0.1:19901"

func Run(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	// Single-instance guard: if another process already holds the port, exit.
	ln, err := net.Listen("tcp", lockPort)
	if err != nil {
		return fmt.Errorf("single-instance lock %s: %w", lockPort, err)
	}
	defer ln.Close()

	if err := config.LoadBootstrap("config.toml"); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	services.DataDir = config.Current().Bot.DataDir
	if err := os.MkdirAll(services.DataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}
	database, err := db.Open(config.Current().Database)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		return err
	}
	drained := true
	defer func() {
		if drained {
			_ = sqlDB.Close()
		}
	}()
	configuration := runtimeconfig.New(runtimeconfig.NewPostgresStore(database), image.ValidateConfiguration)
	if err := configuration.Initialize(); err != nil {
		return fmt.Errorf("runtime config: %w", err)
	}
	if sqlDB, err := database.DB(); err == nil {
		c := config.Current().Database
		sqlDB.SetMaxOpenConns(c.MaxOpen)
		sqlDB.SetMaxIdleConns(c.MaxIdle)
		sqlDB.SetConnMaxLifetime(time.Duration(c.ConnMaxLifetime) * time.Minute)
	}
	bot.SetCommandOptionsProvider(func() (string, bool) { c := config.Current().Bot; return c.CmdPrefix, c.CommandArgSpaceRequired })
	reload := make(chan struct{}, 1)
	var reloadOnce sync.Once

	bot.CmdPrefix = config.Current().Bot.CmdPrefix
	bot.CommandArgSpaceRequired = config.Current().Bot.CommandArgSpaceRequired
	httpclient.InitProxy()

	b := bot.New()
	runtime := newRuntime(ctx, b)
	groups := db.NewGroupStore(database)
	b.SetPluginGate(groups.IsGroupPluginDisabled)
	usage := db.NewCommandUsageStore(database)
	games := db.NewGameStore(database)
	b.SetCommandUsageRecorder(usage.RecordCommandUsage)
	schedules := scheduler.New(db.NewScheduleStore(database))
	b.OnConnect(schedules.Init)

	// ── Smoke test ───────────────────────────────────────────────────────────
	b.OnCommand("ping").Plugin(catalog.PluginSystemTools).Handle(func(ctx *bot.CommandContext) error {
		return ctx.Reply("pong!")
	})

	// ── Group management ─────────────────────────────────────────────────────
	group.RegisterBan(b)
	group.RegisterRevoke(b)
	group.RegisterEssence(b)
	group.RegisterMuteAll(b)
	group.RegisterJoinReview(b, groups)
	group.RegisterMemberBackup(b)
	group.RegisterFiles(b)

	// ── Random ───────────────────────────────────────────────────────────────
	random.RegisterMember(b)
	random.RegisterRename(b)
	random.RegisterRoll(b)

	// ── Image（配置表驱动）+ 语录/表情（独立插件）─────────────────────────────
	// image.Register 必须在 system.RegisterHelp 之前：后者据 activeEntries 生成帮助。
	if err := image.Register(b); err != nil {
		return fmt.Errorf("image configuration: %w", err)
	}
	quotation.Register(b)
	emoticon.Register(b)
	avatar_meme.Register(b)
	imageops.Register(b)

	// ── System ───────────────────────────────────────────────────────────────
	system.RegisterHelp(b)
	b.Go(system.PreRenderHelpImage)
	system.RegisterReboot(b, func() {
		select {
		case reload <- struct{}{}:
		default:
		}
	})
	usageStore := db.NewModelUsageStore(database)
	llm.SetDefaultUsageObserver(func(usage llm.UsageRecord) {
		saveCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if err := usageStore.Record(saveCtx, 0, usage); err != nil {
			logx.Warnf("[ai] background usage persistence failed: %v", err)
		}
	})
	defer llm.SetDefaultUsageObserver(nil)
	affinityStore := db.NewAffinityStore(database)
	engine := ai.NewEngine(ai.EngineDependencies{ModelUsage: db.NewModelUsageStore(database), Conversations: db.NewConversationStore(database), Groups: groups, Memory: ai.NewMemoryManager(database), Chats: db.NewChatStore(database), Affinity: ai.NewAffinityManager(affinityStore)})
	system.RegisterRules(b, engine.Memory)

	// ── Memo ─────────────────────────────────────────────────────────────────
	memo.Register(b, schedules)
	feeds := feed.NewManager(db.NewFeedStore(database), nil)
	subscription.Register(b, feeds)
	knowledgeManager := knowledge.New(db.NewKnowledgeStore(database))
	aitools.RegisterAll(engine.Registry, aitools.Dependencies{Chats: engine.Chats, Memory: engine.Memory, Feeds: feeds, Schedules: schedules, Knowledge: knowledgeManager, Todos: db.NewTodoStore(database)})
	bilibiliManager, stopBilibili := bilibili.Register(b, bservice.NewPostgresStore(database))
	defer stopBilibili()
	configuration.OnApply(func(old, next config.Config, needsReload bool) {
		if sqlDB, err := database.DB(); err == nil {
			c := next.Database
			sqlDB.SetMaxOpenConns(c.MaxOpen)
			sqlDB.SetMaxIdleConns(c.MaxIdle)
			sqlDB.SetConnMaxLifetime(time.Duration(c.ConnMaxLifetime) * time.Minute)
		}
		if bilibiliManager != nil {
			bilibiliManager.SetFallbackRecipients(next.Bot.SuperUsers)
		}
		if needsReload {
			reloadOnce.Do(func() { time.AfterFunc(time.Second, func() { reload <- struct{}{} }) })
		}
	})
	groupknowledge.Register(b, engine, knowledgeManager)

	// ── Game ─────────────────────────────────────────────────────────────────
	game.RegisterCheckIn(b, games)
	game.RegisterScore(b, games)
	game.RegisterRanking(b, games)
	game.RegisterJW3(b)
	game.RegisterDeals(b)

	// ── Fun ──────────────────────────────────────────────────────────────────
	funny.RegisterPoke(b)
	funny.RegisterRepeater(b)
	funny.RegisterSleep(b)
	funny.RegisterHot(b)
	funny.RegisterChatStats(b, db.NewChatStore(database))
	funny.RegisterFortune(b)
	funny.RegisterTraceMoe(b)

	if err := meme.Init(config.Current().Tools.MemeServer); err != nil {
		logx.Warnf("[meme] skipped: %v", err)
	} else {
		funny.RegisterMemes(b)
	}

	// ── Tools ────────────────────────────────────────────────────────────────
	tools.RegisterClockin(b, games)
	tools.RegisterLinkAnalysis(b)
	tools.RegisterZssm(b, engine)
	tools.RegisterPack(b)

	// ── AI dispatch (lowest priority — fires after all specific handlers) ────
	ai_dispatch.Register(b, engine)

	// ── Proactive speech (fires on all messages, lowest priority) ────────────
	ai_proactive.Register(b, engine)

	// ── External HTTP API ─────────────────────────────────────────────────────
	if config.Current().HTTPAPI.Addr != "" {
		srv := httpapi.New(config.Current().HTTPAPI.Key)
		srv.BindBot(b) // 注册 OnConnect 钩子，刷新活的 BotAPI
		runtime.start("httpapi", func() error { return srv.Run(runtime.ctx, config.Current().HTTPAPI.Addr) })
		logx.Infof("[httpapi] enabled on %s", config.Current().HTTPAPI.Addr)
	}

	// ── WebUI Admin ──────────────────────────────────────────────────────────
	if config.Current().WebUI.Enabled {
		srv := webui.New(config.Current().WebUI, webui.Dependencies{ModelUsage: db.NewModelUsageStore(database),
			Images:   imageassets.NewLibrary(services.DataDir, image.AssetCategories(config.Current().Image.Entry)),
			Bilibili: bilibiliManager, Configuration: configuration, Feeds: feeds, Knowledge: knowledgeManager,
			Memory: engine.Memory, Chats: engine.Chats, Affinity: affinityStore, Usage: usage,
			Groups: groups, Overview: db.NewOverviewStore(database), Schedules: schedules,
		})
		srv.BindBot(b)
		runtime.start("webui", func() error { return srv.Run(runtime.ctx, config.Current().WebUI.Addr) })
		logx.Infof("[webui] enabled on %s", config.Current().WebUI.Addr)
	}

	// ── Connect ──────────────────────────────────────────────────────────────
	nc := config.Current().NapCat
	runtime.start("napcat", func() error {
		if nc.Serve != "" {
			return b.ServeContext(runtime.ctx, nc.Serve, nc.Token)
		}
		return b.StartContext(runtime.ctx, nc.URL, nc.Token)
	})

	runtime.cancelWorkers = func() { stopBilibili(); feeds.Cancel(); schedules.Cancel() }
	runtime.waitWorkers = func() { feeds.Stop(); schedules.Stop() }
	drained, err = runtime.wait(reload, 30*time.Second)
	return err
}
