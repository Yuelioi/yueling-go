package wuxia

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	mathrand "math/rand/v2"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Yuelioi/yueling-go/util"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RuleError struct{ Message string }

func (e *RuleError) Error() string           { return e.Message }
func rulef(format string, args ...any) error { return &RuleError{fmt.Sprintf(format, args...)} }

var ErrNotJoined = errors.New("还没有侠客，请先发送「江湖」")

type Service struct {
	database *gorm.DB
	catalog  Catalog
	now      func() time.Time
}

func New(database *gorm.DB) *Service {
	return &Service{database: database, catalog: DefaultCatalog(), now: util.Now}
}
func (s *Service) Catalog() Catalog { return DefaultCatalog() }

type operation struct {
	ID            string
	BotID         int64
	GroupID       int64
	MessageID     int32
	UserID        int64
	Action        string
	Argument      string
	TargetID      int64
	SchemaVersion int
	Payload       string `gorm:"type:jsonb"`
	CreatedAt     time.Time
}

func (operation) TableName() string { return "wuxia_operations" }

type LedgerEntry struct {
	ID          int64     `json:"id"`
	OperationID string    `json:"operation_id"`
	GroupID     int64     `json:"group_id"`
	UserID      int64     `json:"user_id"`
	Delta       int64     `json:"delta"`
	Balance     int64     `json:"balance"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

func (LedgerEntry) TableName() string { return "wuxia_ledger" }

type settingRow struct {
	GroupID   int64    `gorm:"primaryKey;autoIncrement:false"`
	Value     Settings `gorm:"serializer:json;type:jsonb"`
	UpdatedAt time.Time
}

func (settingRow) TableName() string { return "wuxia_settings" }

func settingsIn(tx *gorm.DB, group int64) (Settings, error) {
	settings := DefaultSettings()
	for _, id := range []int64{0, group} {
		var row settingRow
		err := tx.First(&row, "group_id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return settings, err
		}
		settings = row.Value
	}
	settings.DailyRuns = 3
	settings.SignSilver = 0
	return settings, settings.Validate()
}
func (s *Service) Settings(ctx context.Context, group int64) (Settings, error) {
	return settingsIn(s.database.WithContext(ctx), group)
}
func (s *Service) SetSettings(ctx context.Context, group int64, settings Settings) error {
	if group < 0 {
		return rulef("群号无效")
	}
	if err := settings.Validate(); err != nil {
		return err
	}
	return s.database.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "group_id"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"})}).Create(&settingRow{GroupID: group, Value: settings, UpdatedAt: s.now()}).Error
}
func (s *Service) ResetSettings(ctx context.Context, group int64) error {
	if group <= 0 {
		return rulef("仅单群配置可恢复继承")
	}
	return s.database.WithContext(ctx).Delete(&settingRow{}, "group_id = ?", group).Error
}

func (s *Service) Profile(ctx context.Context, group, user int64) (View, Daily, error) {
	tx := s.database.WithContext(ctx)
	var p Character
	if err := tx.First(&p, "group_id = ? AND user_id = ?", group, user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return View{}, Daily{}, ErrNotJoined
		}
		return View{}, Daily{}, err
	}
	settings, err := settingsIn(tx, group)
	if err != nil {
		return View{}, Daily{}, err
	}
	v, err := s.catalog.View(p, settings)
	if err != nil {
		return v, Daily{}, err
	}
	d := Daily{GroupID: group, UserID: user, Day: s.now().Format("2006-01-02")}
	err = tx.Where("group_id = ? AND user_id = ? AND day = ?", group, user, d.Day).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return v, d, err
}

func (s *Service) operate(ctx context.Context, req Request, command Command) (Result, error) {
	if req.BotID <= 0 || req.GroupID <= 0 || req.UserID <= 0 || req.MessageID == 0 {
		return Result{}, rulef("缺少真实消息身份")
	}
	if _, ok := actionHandlers[req.Action]; !ok {
		return Result{}, rulef("未知江湖操作")
	}
	var result Result
	err := s.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		h := fnv.New64a()
		fmt.Fprintf(h, "wuxia:%d:%d:%d", req.BotID, req.GroupID, req.MessageID)
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(h.Sum64())).Error; err != nil {
			return err
		}
		var receipt operation
		err := tx.First(&receipt, "bot_id = ? AND group_id = ? AND message_id = ?", req.BotID, req.GroupID, req.MessageID).Error
		if err == nil {
			if receipt.UserID != req.UserID || receipt.Action != req.Action || receipt.TargetID != req.TargetID || receipt.Argument != req.Argument {
				return rulef("消息已用于另一项操作")
			}
			if receipt.SchemaVersion != 1 {
				return rulef("江湖回执版本不支持")
			}
			result, err = decodeReceipt(receipt.Payload)
			if err != nil {
				return err
			}
			result.Replayed = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := s.now()
		settings, err := settingsIn(tx, req.GroupID)
		if err != nil {
			return err
		}
		if req.Action == "duel" && (req.TargetID <= 0 || req.TargetID == req.UserID || req.TargetID == req.BotID) {
			return rulef("请艾特另一位群友")
		}
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return err
		}
		rng := mathrand.New(mathrand.NewPCG(binary.LittleEndian.Uint64(random[:]), 0xa0761d6478bd642f))
		participants := []int64{req.UserID}
		if req.Action == "duel" {
			participants = append(participants, req.TargetID)
			sort.Slice(participants, func(i, j int) bool { return participants[i] < participants[j] })
		}
		var p, target Character
		joined, targetJoined := false, false
		// Reciprocal duels must insert and lock both characters in the same order.
		for _, userID := range participants {
			name := req.Name
			if userID != req.UserID {
				name = req.TargetName
			}
			var character Character
			createdNow := false
			if req.Action == "daily" || req.Action == "adventure" || req.Action == "duel" {
				character = s.catalog.newCharacter(req.GroupID, userID, name, now, rng)
				created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&character)
				if created.Error != nil {
					return created.Error
				}
				createdNow = created.RowsAffected == 1
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&character, "group_id = ? AND user_id = ?", req.GroupID, userID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrNotJoined
				}
				return err
			}
			if userID == req.UserID {
				p, joined = character, createdNow
			} else {
				target, targetJoined = character, createdNow
			}
		}
		p.Name = cleanName(req.Name)
		inventoryBefore := maps.Clone(p.Inventory)
		before, err := s.catalog.View(p, settings)
		if err != nil {
			return err
		}
		d := Daily{GroupID: p.GroupID, UserID: p.UserID, Day: now.Format("2006-01-02")}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&d).Error; err != nil {
			return err
		}
		if err := tx.First(&d, "group_id = ? AND user_id = ? AND day = ?", d.GroupID, d.UserID, d.Day).Error; err != nil {
			return err
		}
		result = Result{Version: s.catalog.Version, DailyLimit: settings.DailyRuns, Joined: joined, TargetJoined: targetJoined, SettledAt: now}
		if joined {
			result.Lines = append(result.Lines, "初入江湖 · 已创建"+before.Profession.Name+"，发送「江湖转职」可免费选择职业")
		}

		if req.Action == "duel" {
			var count int64
			if err := tx.Model(&operation{}).Where("group_id = ? AND user_id = ? AND action = 'duel' AND target_id = ? AND created_at > ?", req.GroupID, req.UserID, req.TargetID, now.Add(-30*time.Second)).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return rulef("刚刚与这位群友切磋过，30秒后再来")
			}

		}
		if err := actionHandlers[req.Action](&gameAction{catalog: s.catalog, command: command, player: &p, request: req, result: &result, before: before, settings: settings, daily: &d, rng: rng, target: target, targetJoined: targetJoined}); err != nil {
			return err
		}

		p.UpdatedAt = now
		if err := tx.Save(&p).Error; err != nil {
			return err
		}
		if err := tx.Save(&d).Error; err != nil {
			return err
		}
		result.Actor, err = s.catalog.View(p, settings)
		if err != nil {
			return err
		}
		if before.Synergy != result.Actor.Synergy {
			if before.Synergy != "" {
				result.Lines = append(result.Lines, before.Synergy+"解除")
			}
			if result.Actor.Synergy != "" {
				result.Lines = append(result.Lines, result.Actor.Synergy+"生效")
			}
		}
		result.InventoryDelta = map[string]int64{}
		for id, n := range p.Inventory {
			if n != inventoryBefore[id] {
				result.InventoryDelta[id] = n - inventoryBefore[id]
			}
		}
		result.Daily = d
		result.SilverDelta = p.Silver - before.Character.Silver
		result.ExperienceDelta = p.Experience - before.Character.Experience
		if result.Actor.Level > before.Level {
			result.Lines = append(result.Lines, fmt.Sprintf("等级 %d → %d，战力 %d → %d", before.Level, result.Actor.Level, before.Power, result.Actor.Power))
		}
		if len(result.Lines) == 0 {
			if req.Action == "adventure" {
				result.Lines = append(result.Lines, "本次历练已结算")
			} else {
				result.Lines = append(result.Lines, "今日签到已完成")
			}
		}
		payload, err := json.Marshal(result)
		if err != nil {
			return err
		}
		receipt = operation{ID: uuid.NewString(), BotID: req.BotID, GroupID: req.GroupID, MessageID: req.MessageID, UserID: req.UserID, Action: req.Action, Argument: req.Argument, TargetID: req.TargetID, SchemaVersion: 1, Payload: string(payload), CreatedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		if result.SilverDelta != 0 {
			return tx.Create(&LedgerEntry{OperationID: receipt.ID, GroupID: p.GroupID, UserID: p.UserID, Delta: result.SilverDelta, Balance: p.Silver, Reason: req.Action, CreatedAt: now}).Error
		}
		return nil
	})
	return result, err
}

func (s *Service) LatestResult(ctx context.Context, bot, group, user int64) (Result, error) {
	var saved operation
	err := s.database.WithContext(ctx).Where("bot_id = ? AND group_id = ? AND user_id = ?", bot, group, user).Order("sequence DESC").First(&saved).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Result{}, rulef("还没有江湖战报，发送「江湖」即可开始")
	}
	if err != nil {
		return Result{}, err
	}
	if saved.SchemaVersion != 1 {
		return Result{}, rulef("江湖回执版本不支持")
	}
	result, err := decodeReceipt(saved.Payload)
	if err != nil {
		return Result{}, err
	}
	result.Replayed = true
	return result, nil
}

func cleanName(name string) string {
	r := []rune(strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)))
	if len(r) > 24 {
		r = r[:24]
	}
	if len(r) == 0 {
		return "无名侠客"
	}
	return string(r)
}

func (s *Service) Ranking(ctx context.Context, group int64, n int) ([]View, error) {
	var players []Character
	tx := s.database.WithContext(ctx)
	if err := tx.Where("group_id = ?", group).Find(&players).Error; err != nil {
		return nil, err
	}
	settings, err := settingsIn(tx, group)
	if err != nil {
		return nil, err
	}
	views := make([]View, 0, len(players))
	for _, p := range players {
		v, err := s.catalog.View(p, settings)
		if err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Power == views[j].Power {
			return views[i].Character.UserID < views[j].Character.UserID
		}
		return views[i].Power > views[j].Power
	})
	return views[:min(len(views), max(1, min(n, 100)))], nil
}
func (s *Service) Ledger(ctx context.Context, group, user int64) ([]LedgerEntry, error) {
	rows := []LedgerEntry{}
	q := s.database.WithContext(ctx).Where("group_id = ?", group)
	if user > 0 {
		q = q.Where("user_id = ?", user)
	}
	err := q.Order("id DESC").Limit(50).Find(&rows).Error
	return rows, err
}

type Configuration struct {
	Settings Settings `json:"settings"`
	Global   Settings `json:"global"`
	Custom   bool     `json:"custom"`
}

func (s *Service) Configuration(ctx context.Context, group int64) (Configuration, error) {
	global, err := s.Settings(ctx, 0)
	if err != nil {
		return Configuration{}, err
	}
	effective, err := s.Settings(ctx, group)
	if err != nil {
		return Configuration{}, err
	}
	var count int64
	err = s.database.WithContext(ctx).Model(&settingRow{}).Where("group_id = ?", group).Count(&count).Error
	return Configuration{Settings: effective, Global: global, Custom: count > 0}, err
}

// Called only for an insert candidate. On conflict the persisted character wins.
func (c Catalog) newCharacter(groupID, userID int64, name string, now time.Time, rng *mathrand.Rand) Character {
	prof := c.Professions[rng.IntN(len(c.Professions))]
	starter := c.StarterFor(prof.ID)
	return Character{GroupID: groupID, UserID: userID, Name: cleanName(name), ProfessionID: prof.ID, WeaponKind: starter.Kind, Weapons: map[string]string{starter.Kind: starter.ID}, Inventory: map[string]int64{starter.ID: 1}, Equipment: map[string]string{}, LearnedSkills: map[string]bool{}, Draws: map[string]int64{}, CreatedAt: now, UpdatedAt: now}
}
