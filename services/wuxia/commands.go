package wuxia

import (
	"context"
	"strconv"
	"strings"
)

type Envelope struct {
	BotID, GroupID, UserID int64
	MessageID              int32
	Name, TargetName       string
}

type Command interface {
	request() (string, string, int64)
}
type DailyCommand struct{ Mode string }
type TrainCommand struct{}
type CasinoCommand struct {
	Stake int64
}

func (c CasinoCommand) request() (string, string, int64) {
	return "casino", strconv.FormatInt(c.Stake, 10), 0
}

type ProfessionCommand struct{ ID string }
type WeaponCommand struct{ Reference string }
type EquipCommand struct{ Reference string }
type LearnCommand struct{ Reference string }
type SkillCommand struct{ Reference string }
type PortraitCommand struct{ Reference string }
type DuelCommand struct{ TargetID int64 }
type DrawCommand struct {
	PoolID string
	Count  int
}

func (c DailyCommand) request() (string, string, int64)      { return c.Mode, "", 0 }
func (TrainCommand) request() (string, string, int64)        { return "train", "", 0 }
func (c ProfessionCommand) request() (string, string, int64) { return "profession", c.ID, 0 }
func (c WeaponCommand) request() (string, string, int64)     { return "weapon", c.Reference, 0 }
func (c EquipCommand) request() (string, string, int64)      { return "equip", c.Reference, 0 }
func (c LearnCommand) request() (string, string, int64)      { return "learn", c.Reference, 0 }
func (c SkillCommand) request() (string, string, int64)      { return "skill", c.Reference, 0 }
func (c PortraitCommand) request() (string, string, int64)   { return "portrait", c.Reference, 0 }
func (c DuelCommand) request() (string, string, int64)       { return "duel", "", c.TargetID }
func (c DrawCommand) request() (string, string, int64) {
	return "draw", c.PoolID + " " + strconv.Itoa(c.Count), 0
}

func ParseCommand(action, argument string, target int64) (Command, error) {
	switch action {
	case "daily", "sign", "adventure":
		if argument != "" {
			return nil, rulef("此操作不需要额外参数")
		}
		return DailyCommand{action}, nil
	case "casino":
		args := strings.Fields(argument)
		if len(args) > 1 {
			return nil, rulef("用法：江湖赌场 100（最低20银两）")
		}
		stake := int64(20)
		if len(args) == 1 {
			var err error
			stake, err = strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return nil, rulef("下注银两须为整数")
			}
		}
		if _, err := ResolveCasino(stake, [3]int{1, 2, 3}); err != nil {
			return nil, err
		}
		return CasinoCommand{Stake: stake}, nil
	case "train":
		if argument != "" {
			return nil, rulef("练武不需要额外参数")
		}
		return TrainCommand{}, nil
	case "profession":
		return ProfessionCommand{argument}, nil
	case "weapon":
		return WeaponCommand{argument}, nil
	case "equip":
		return EquipCommand{argument}, nil
	case "learn":
		return LearnCommand{argument}, nil
	case "skill":
		return SkillCommand{argument}, nil
	case "portrait":
		return PortraitCommand{argument}, nil
	case "duel":
		return DuelCommand{target}, nil
	case "draw":
		cmd := DrawCommand{PoolID: "jianghu", Count: 1}
		args := strings.Fields(argument)
		if len(args) > 2 {
			return nil, rulef("用法：江湖寻宝 [奖池名] [1或5]")
		}
		hasPool, hasCount := false, false
		for _, arg := range args {
			if n, err := strconv.Atoi(arg); err == nil {
				if hasCount {
					return nil, rulef("抽取次数只能填写一次")
				}
				hasCount = true
				cmd.Count = n
			} else {
				if hasPool {
					return nil, rulef("奖池名称只能填写一次")
				}
				hasPool = true
				cmd.PoolID = arg
			}
		}
		if cmd.Count != 1 && cmd.Count != 5 {
			return nil, rulef("每次可寻宝1次或5次")
		}
		return cmd, nil
	default:
		return nil, rulef("未知江湖操作")
	}
}

func (s *Service) Execute(ctx context.Context, e Envelope, command Command) (Result, error) {
	if command == nil {
		return Result{}, rulef("缺少江湖操作")
	}
	action, argument, target := command.request()
	checked, err := ParseCommand(action, argument, target)
	if err != nil {
		return Result{}, err
	}
	return s.operate(ctx, Request{BotID: e.BotID, GroupID: e.GroupID, UserID: e.UserID, MessageID: e.MessageID, Name: e.Name, TargetName: e.TargetName, Action: action, Argument: argument, TargetID: target}, checked)
}

func (s *Service) Operate(ctx context.Context, req Request) (Result, error) {
	command, err := ParseCommand(req.Action, req.Argument, req.TargetID)
	if err != nil {
		return Result{}, err
	}
	return s.operate(ctx, req, command)
}
