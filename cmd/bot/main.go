package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
	"github.com/Yuelioi/yueling-go/services/httpapi"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/Yuelioi/yueling-go/services/meme"
	"github.com/Yuelioi/yueling-go/services/runtimeconfig"
	"github.com/Yuelioi/yueling-go/services/webui"

	// AI tools register themselves via init()
	_ "github.com/Yuelioi/yueling-go/ai/tools"
)

const lockPort = "127.0.0.1:19901"

func main() {
	// Single-instance guard: if another process already holds the port, exit.
	ln, err := net.Listen("tcp", lockPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[bot] another instance is already running (port %s busy), exiting.\n", lockPort)
		os.Exit(1)
	}
	defer ln.Close()

	if err := config.LoadBootstrap("config.toml"); err != nil {
		logx.Fatalf("config: %v", err)
	}

	services.DataDir = config.Current().Bot.DataDir
	if err := os.MkdirAll(services.DataDir, 0o755); err != nil {
		logx.Fatalf("mkdir data: %v", err)
	}
	if err := db.Init(config.Current().Database); err != nil {
		logx.Fatalf("db: %v", err)
	}

	configuration := runtimeconfig.New(runtimeconfig.PostgresStore{}, image.ValidateConfiguration)
	if err := configuration.Initialize(); err != nil {
		logx.Fatalf("runtime config: %v", err)
	}
	if sqlDB, err := db.DB.DB(); err == nil {
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
	b.SetPluginGate(db.IsGroupPluginDisabled)
	b.SetCommandUsageRecorder(db.RecordCommandUsage)
	b.OnConnect(scheduler.Init)

	// ── Smoke test ───────────────────────────────────────────────────────────
	b.OnCommand("ping").Plugin(catalog.PluginSystemTools).Handle(func(ctx *bot.CommandContext) error {
		return ctx.Reply("pong!")
	})

	// ── Group management ─────────────────────────────────────────────────────
	group.RegisterBan(b)
	group.RegisterRevoke(b)
	group.RegisterEssence(b)
	group.RegisterMuteAll(b)
	group.RegisterJoinReview(b)
	group.RegisterMemberBackup(b)
	group.RegisterFiles(b)

	// ── Random ───────────────────────────────────────────────────────────────
	random.RegisterMember(b)
	random.RegisterRename(b)
	random.RegisterRoll(b)

	// ── Image（配置表驱动）+ 语录/表情（独立插件）─────────────────────────────
	// image.Register 必须在 system.RegisterHelp 之前：后者据 activeEntries 生成帮助。
	image.Register(b)
	quotation.Register(b)
	emoticon.Register(b)
	avatar_meme.Register(b)
	imageops.Register(b)

	// ── System ───────────────────────────────────────────────────────────────
	system.RegisterHelp(b)
	system.RegisterReboot(b, config.Current().Bot.SuperUsers)
	system.RegisterRules(b)

	// ── Memo ─────────────────────────────────────────────────────────────────
	memo.Register(b)
	subscription.Register(b)
	bilibiliManager, stopBilibili := bilibili.Register(b)
	defer stopBilibili()
	configuration.OnApply(func(old, next config.Config, needsReload bool) {
		if sqlDB, err := db.DB.DB(); err == nil {
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
	groupknowledge.Register(b)

	// ── Game ─────────────────────────────────────────────────────────────────
	game.RegisterCheckIn(b)
	game.RegisterScore(b)
	game.RegisterRanking(b)
	game.RegisterJW3(b)
	game.RegisterDeals(b)

	// ── Fun ──────────────────────────────────────────────────────────────────
	funny.RegisterPoke(b)
	funny.RegisterRepeater(b)
	funny.RegisterSleep(b)
	funny.RegisterHot(b)
	funny.RegisterChatStats(b)
	funny.RegisterFortune(b)
	funny.RegisterTraceMoe(b)

	if err := meme.Init(config.Current().Tools.MemeServer); err != nil {
		logx.Warnf("[meme] skipped: %v", err)
	} else {
		funny.RegisterMemes(b)
	}

	// ── Tools ────────────────────────────────────────────────────────────────
	tools.RegisterClockin(b)
	tools.RegisterLinkAnalysis(b)
	tools.RegisterZssm(b)
	tools.RegisterPack(b)

	// ── AI dispatch (lowest priority — fires after all specific handlers) ────
	ai_dispatch.Register(b)

	// ── Proactive speech (fires on all messages, lowest priority) ────────────
	ai_proactive.Register(b)

	// ── External HTTP API ─────────────────────────────────────────────────────
	if config.Current().HTTPAPI.Addr != "" {
		srv := httpapi.New(config.Current().HTTPAPI.Key)
		srv.BindBot(b) // 注册 OnConnect 钩子，刷新活的 BotAPI
		go srv.Start(config.Current().HTTPAPI.Addr)
		logx.Infof("[httpapi] enabled on %s", config.Current().HTTPAPI.Addr)
	}

	// ── WebUI Admin ──────────────────────────────────────────────────────────
	if config.Current().WebUI.Enabled {
		srv := webui.New(config.Current().WebUI)
		srv.BindBilibili(bilibiliManager)
		srv.BindConfiguration(configuration)
		srv.BindBot(b)
		go srv.Start(config.Current().WebUI.Addr)
		logx.Infof("[webui] enabled on %s", config.Current().WebUI.Addr)
	}

	// ── Connect ──────────────────────────────────────────────────────────────
	nc := config.Current().NapCat
	go func() {
		if nc.Serve != "" {
			b.Serve(nc.Serve, nc.Token)
		} else {
			b.Start(nc.URL, nc.Token)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-quit:
	case <-reload:
		logx.Infof("[config] saved; restarting supervised service")
	}
	logx.Infof("[bot] shutting down...")
	stopBilibili()
	if sqlDB, err := db.DB.DB(); err == nil {
		sqlDB.Close()
	}
}
