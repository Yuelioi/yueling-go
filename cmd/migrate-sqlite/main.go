package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"

	"github.com/Yuelioi/yueling-go/config"
	appdb "github.com/Yuelioi/yueling-go/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type tableCopy struct {
	name        string
	rows        any
	legacyQuery string
}

func main() {
	sqlitePath := flag.String("sqlite", "data/yueling.db", "旧 SQLite 数据库路径")
	postgresDSN := flag.String("postgres", os.Getenv("YUELING_DATABASE_DSN"), "目标 PostgreSQL DSN（默认读取 YUELING_DATABASE_DSN）")
	flag.Parse()
	if *postgresDSN == "" {
		fatalf("必须通过 --postgres 或 YUELING_DATABASE_DSN 提供 PostgreSQL DSN")
	}
	sqliteAbsolutePath, err := filepath.Abs(*sqlitePath)
	if err != nil {
		fatalf("解析 SQLite 路径: %v", err)
	}
	if _, err := os.Stat(sqliteAbsolutePath); err != nil {
		fatalf("读取 SQLite 文件: %v", err)
	}

	sqliteURL := (&url.URL{Scheme: "file", Path: sqliteAbsolutePath, RawQuery: "mode=ro"}).String()
	source, err := gorm.Open(sqlite.Open(sqliteURL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fatalf("打开 SQLite: %v", err)
	}
	sourceSQL, _ := source.DB()
	defer sourceSQL.Close()

	if err := appdb.Init(config.DatabaseConfig{
		DSN: *postgresDSN, MaxOpen: 5, MaxIdle: 1, ConnMaxLifetime: 10,
	}); err != nil {
		fatalf("初始化 PostgreSQL: %v", err)
	}
	targetSQL, _ := appdb.DB.DB()
	defer targetSQL.Close()

	tables := []tableCopy{
		{"user_game_records", &[]appdb.UserGameRecord{}, ""},
		{"ai_affinities", &[]appdb.AIAffinity{}, ""},
		{"reminders", &[]appdb.Reminder{}, ""},
		{"user_profiles", &[]appdb.UserProfile{}, ""},
		{"todo_items", &[]appdb.TodoItem{}, ""},
		{"semantic_memories", &[]appdb.SemanticMemory{}, ""},
		{"procedural_memories", &[]appdb.ProceduralMemory{}, ""},
		{"group_join_rules", &[]appdb.GroupJoinRule{}, ""},
		{"group_plugin_disableds", &[]appdb.GroupPluginDisabled{}, ""},
		{"daily_digests", &[]appdb.DailyDigest{}, ""},
		{"feed_subscriptions", &[]appdb.FeedSubscription{}, ""},
		{"group_knowledges", &[]appdb.GroupKnowledge{}, ""},
		{"group_chat_messages", &[]appdb.GroupChatMessage{}, ""},
	}

	for _, table := range tables {
		if !source.Migrator().HasTable(table.name) {
			fmt.Printf("%-28s 跳过（旧库无此表）\n", table.name)
			continue
		}
		query := source.Table(table.name)
		if table.legacyQuery != "" {
			query = source.Raw(table.legacyQuery)
		}
		if err := query.Scan(table.rows).Error; err != nil {
			fatalf("读取 %s: %v", table.name, err)
		}
		count := reflect.ValueOf(table.rows).Elem().Len()
		if count > 0 {
			if err := appdb.DB.Table(table.name).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(table.rows, 500).Error; err != nil {
				fatalf("写入 %s: %v", table.name, err)
			}
		}
		if err := resetSequence(appdb.DB, table.name); err != nil {
			fatalf("重置 %s 序列: %v", table.name, err)
		}
		fmt.Printf("%-28s %d 行\n", table.name, count)
	}

	fmt.Println("SQLite 数据迁移完成；请核对后再归档旧 yueling.db。")
}

func resetSequence(database *gorm.DB, table string) error {
	// table comes exclusively from the fixed list above, not from user input.
	query := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE(MAX(id), 1), COUNT(*) > 0) FROM %s`, table, table)
	return database.Exec(query).Error
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate-sqlite: "+format+"\n", args...)
	os.Exit(1)
}
