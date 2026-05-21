package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/plugins/ai_dispatch"
	"github.com/Yuelioi/yueling-go/plugins/ai_proactive"
	"github.com/Yuelioi/yueling-go/plugins/funny"
	"github.com/Yuelioi/yueling-go/plugins/game"
	"github.com/Yuelioi/yueling-go/plugins/group"
	"github.com/Yuelioi/yueling-go/plugins/memo"
	"github.com/Yuelioi/yueling-go/plugins/random"
	"github.com/Yuelioi/yueling-go/plugins/system"
	"github.com/Yuelioi/yueling-go/plugins/tools"
	"github.com/Yuelioi/yueling-go/plugins/user"
	"github.com/Yuelioi/yueling-go/scheduler"
	"github.com/Yuelioi/yueling-go/services"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/meme"

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

	if err := config.Load("config.toml"); err != nil {
		log.Fatalf("config: %v", err)
	}

	ai := config.C.AI
	log.Printf("[config] model=%s base_url=%s key=****", ai.Model, ai.BaseURL)
	log.Printf("[config] napcat=%s", config.C.NapCat.URL)

	services.DataDir = config.C.Bot.DataDir
	if err := os.MkdirAll(services.DataDir, 0o755); err != nil {
		log.Fatalf("mkdir data: %v", err)
	}
	if err := db.Init(services.DataPath("yueling.db")); err != nil {
		log.Fatalf("db: %v", err)
	}

	bot.CmdPrefix = config.C.Bot.CmdPrefix
	httpclient.InitProxy()

	b := bot.New()
	b.OnConnect(scheduler.Init)

	// ── Smoke test ───────────────────────────────────────────────────────────
	b.OnCommand("ping").Handle(func(ctx *bot.CommandContext) error {
		return ctx.Reply("pong!")
	})

	// ── Group management ─────────────────────────────────────────────────────
	group.RegisterBan(b)
	group.RegisterRevoke(b)
	group.RegisterMuteAll(b)
	group.RegisterKeyword(b)
	group.RegisterManager(b)
	group.RegisterMemberBackup(b)
	group.RegisterFiles(b)

	// ── Random ───────────────────────────────────────────────────────────────
	random.RegisterMember(b)
	random.RegisterRename(b)
	random.RegisterRoll(b)
	random.RegisterEmoticon(b)
	random.RegisterImage(b)
	random.RegisterQuotation(b)
	random.RegisterDaily(b)

	// ── System ───────────────────────────────────────────────────────────────
	system.RegisterHelp(b)
	system.RegisterReboot(b, config.C.Bot.SuperUsers)
	system.RegisterReply(b)
	system.RegisterRules(b)
	system.RegisterImage(b)

	// ── Memo ─────────────────────────────────────────────────────────────────
	memo.Register(b)

	// ── Game ─────────────────────────────────────────────────────────────────
	game.RegisterCheckIn(b)
	game.RegisterScore(b)
	game.RegisterRanking(b)
	game.RegisterPK(b)
	game.RegisterJW3(b)

	// ── Fun ──────────────────────────────────────────────────────────────────
	funny.RegisterPoke(b)
	funny.RegisterRepeater(b)
	funny.RegisterSleep(b)
	funny.RegisterHot(b)
	funny.RegisterChat(b)
	funny.RegisterFortune(b)
	funny.RegisterTraceMoe(b)

	if err := meme.Init(config.C.Tools.MemeServer); err != nil {
		log.Printf("[meme] skipped: %v", err)
	} else {
		funny.RegisterMemes(b)
	}

	// ── Tools ────────────────────────────────────────────────────────────────
	tools.RegisterTranslate(b)
	tools.RegisterClockin(b)
	tools.RegisterLinkAnalysis(b)
	tools.RegisterSearchAE(b)

	// ── User ─────────────────────────────────────────────────────────────────
	user.Register(b)

	// ── AI dispatch (lowest priority — fires after all specific handlers) ────
	ai_dispatch.Register(b)

	// ── Proactive speech (fires on all messages, lowest priority) ────────────
	ai_proactive.Register(b)

	// ── Connect ──────────────────────────────────────────────────────────────
	nc := config.C.NapCat
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
	<-quit
	log.Println("[bot] shutting down...")
	if sqlDB, err := db.DB.DB(); err == nil {
		sqlDB.Close()
	}
}
