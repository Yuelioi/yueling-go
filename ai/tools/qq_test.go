package tools

import (
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/ai"
)

type qqActionsSpy struct {
	action    string
	groupID   int64
	userID    int64
	messageID int32
	duration  int
	value     string
}

func (s *qqActionsSpy) SetEssenceMsg(messageID int32) error {
	s.action, s.messageID = "set_essence", messageID
	return nil
}

func (s *qqActionsSpy) DeleteEssenceMsg(messageID int32) error {
	s.action, s.messageID = "delete_essence", messageID
	return nil
}

func (s *qqActionsSpy) DeleteMsg(messageID int32) error {
	s.action, s.messageID = "delete_message", messageID
	return nil
}

func (s *qqActionsSpy) SetGroupBan(groupID, userID int64, seconds int) error {
	s.action, s.groupID, s.userID = "ban", groupID, userID
	s.duration = seconds
	return nil
}

func (s *qqActionsSpy) SetGroupCard(groupID, userID int64, card string) error {
	s.action, s.groupID, s.userID, s.value = "set_card", groupID, userID, card
	return nil
}

func (s *qqActionsSpy) SetGroupSpecialTitle(groupID, userID int64, title string) error {
	s.action, s.groupID, s.userID, s.value = "set_title", groupID, userID, title
	return nil
}

func (s *qqActionsSpy) GroupPoke(groupID, userID int64) error {
	s.action, s.groupID, s.userID = "poke", groupID, userID
	return nil
}

func TestHandleSetEssenceUsesReplyMessage(t *testing.T) {
	actions := &qqActionsSpy{}
	inv := qqInvocation{replyMessageID: 300, hasReply: true}

	result, err := handleSetEssence(inv, actions)
	if err != nil {
		t.Fatalf("handleSetEssence() error = %v", err)
	}
	if actions.action != "set_essence" || actions.messageID != 300 {
		t.Fatalf("action = %+v, want set_essence message 300", actions)
	}
	if !strings.Contains(result, "已将") {
		t.Fatalf("result = %q, want success message", result)
	}
}

func TestHandleSetEssenceRequiresReply(t *testing.T) {
	actions := &qqActionsSpy{}

	result, err := handleSetEssence(qqInvocation{}, actions)
	if err != nil {
		t.Fatalf("handleSetEssence() error = %v", err)
	}
	if actions.action != "" {
		t.Fatalf("unexpected action = %q", actions.action)
	}
	if !strings.Contains(result, "请先回复") {
		t.Fatalf("result = %q, want reply guidance", result)
	}
}

func TestRevokeMessageUsesReplyOrHistoryMessageID(t *testing.T) {
	tests := []struct {
		name      string
		inv       qqInvocation
		requested int64
		wantID    int32
	}{
		{"reply", qqInvocation{replyMessageID: 300, hasReply: true}, 0, 300},
		{"history id", qqInvocation{historyMessages: map[int32]bool{400: true}}, 400, 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := &qqActionsSpy{}
			if _, err := handleRevokeMessage(tt.inv, tt.requested, actions); err != nil {
				t.Fatalf("handleRevokeMessage() error = %v", err)
			}
			if actions.action != "delete_message" || actions.messageID != tt.wantID {
				t.Fatalf("action = %+v, want delete message %d", actions, tt.wantID)
			}
		})
	}
}

func TestRevokeMessageRejectsUntrustedMessageID(t *testing.T) {
	actions := &qqActionsSpy{}

	result, err := handleRevokeMessage(qqInvocation{}, 400, actions)
	if err != nil {
		t.Fatalf("handleRevokeMessage() error = %v", err)
	}
	if actions.action != "" {
		t.Fatalf("unexpected action = %q", actions.action)
	}
	if !strings.Contains(result, "不在当前聊天记录") {
		t.Fatalf("result = %q, want untrusted ID guidance", result)
	}
}

func TestBanMemberUsesHistoryUserIDAndDefaultDuration(t *testing.T) {
	actions := &qqActionsSpy{}
	inv := qqInvocation{groupID: 100, botID: 999, historyUsers: map[int64]bool{200: true}}

	result, err := handleBanMember(inv, 200, 0, false, actions)
	if err != nil {
		t.Fatalf("handleBanMember() error = %v", err)
	}
	if actions.action != "ban" || actions.groupID != 100 || actions.userID != 200 || actions.duration != 600 {
		t.Fatalf("action = %+v, want user 200 banned for 600 seconds", actions)
	}
	if !strings.Contains(result, "10 分钟") {
		t.Fatalf("result = %q, want default duration", result)
	}
}

func TestBanMemberRejectsBotItself(t *testing.T) {
	actions := &qqActionsSpy{}
	inv := qqInvocation{groupID: 100, botID: 999}

	result, err := handleBanMember(inv, 999, 600, true, actions)
	if err != nil {
		t.Fatalf("handleBanMember() error = %v", err)
	}
	if actions.action != "" {
		t.Fatalf("unexpected action = %q", actions.action)
	}
	if !strings.Contains(result, "机器人自己") {
		t.Fatalf("result = %q, want self-protection", result)
	}
}

func TestBanMemberRejectsUntrustedUserID(t *testing.T) {
	actions := &qqActionsSpy{}
	inv := qqInvocation{groupID: 100, botID: 999}

	result, err := handleBanMember(inv, 200, 600, true, actions)
	if err != nil {
		t.Fatalf("handleBanMember() error = %v", err)
	}
	if actions.action != "" {
		t.Fatalf("unexpected action = %q", actions.action)
	}
	if !strings.Contains(result, "不在当前消息或聊天记录") {
		t.Fatalf("result = %q, want untrusted ID guidance", result)
	}
}

func TestMemberCanOnlyEditOwnGroupCard(t *testing.T) {
	actions := &qqActionsSpy{}
	inv := qqInvocation{
		groupID:        100,
		callerID:       200,
		permission:     ai.PermMember,
		mentionedUsers: []int64{300},
	}

	result, err := handleSetGroupCard(inv, " 新名字 ", actions)
	if err != nil {
		t.Fatalf("handleSetGroupCard() error = %v", err)
	}
	if actions.action != "" {
		t.Fatalf("unexpected action = %q", actions.action)
	}
	if !strings.Contains(result, "只能修改自己") {
		t.Fatalf("result = %q, want permission denial", result)
	}
}

func TestAdminCanEditMentionedMembersTitle(t *testing.T) {
	actions := &qqActionsSpy{}
	inv := qqInvocation{
		groupID:        100,
		callerID:       200,
		permission:     ai.PermAdmin,
		mentionedUsers: []int64{300},
	}

	result, err := handleSetSpecialTitle(inv, " 摸鱼冠军 ", actions)
	if err != nil {
		t.Fatalf("handleSetSpecialTitle() error = %v", err)
	}
	if actions.action != "set_title" || actions.groupID != 100 || actions.userID != 300 || actions.value != "摸鱼冠军" {
		t.Fatalf("action = %+v, want title update for user 300", actions)
	}
	if !strings.Contains(result, "摸鱼冠军") {
		t.Fatalf("result = %q, want title", result)
	}
}

func TestGroupPokeUsesFirstMentionedMember(t *testing.T) {
	actions := &qqActionsSpy{}
	inv := qqInvocation{groupID: 100, callerID: 200, mentionedUsers: []int64{300, 400}}

	if _, err := handleGroupPoke(inv, actions); err != nil {
		t.Fatalf("handleGroupPoke() error = %v", err)
	}
	if actions.action != "poke" || actions.groupID != 100 || actions.userID != 300 {
		t.Fatalf("action = %+v, want poke user 300", actions)
	}
}

func TestQQActionToolsAreRegisteredForRouting(t *testing.T) {
	for _, name := range []string{
		"set_essence_message",
		"delete_essence_message",
		"revoke_message",
		"ban_member",
		"set_group_card",
		"set_special_title",
		"poke_group_member",
	} {
		tool, ok := ai.GetTool(name)
		if !ok || tool.Handler == nil {
			t.Fatalf("tool %q is not registered", name)
		}
	}
	for _, name := range []string{"revoke_message", "ban_member"} {
		tool, _ := ai.GetTool(name)
		if tool.Permission != ai.PermMember {
			t.Fatalf("tool %q permission = %v, want PermMember", name, tool.Permission)
		}
	}
}

func TestQQActionToolsRouteNaturalRequests(t *testing.T) {
	tests := []struct {
		toolName string
		text     string
	}{
		{"set_essence_message", "把这条设精"},
		{"revoke_message", "撤回刚才那条广告"},
		{"ban_member", "禁言刚才说脏话的人"},
		{"set_group_card", "把我的群名片改成今天不加班"},
		{"set_special_title", "给我一个摸鱼冠军的头衔"},
		{"poke_group_member", "戳一下他"},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			tool, ok := ai.GetTool(tt.toolName)
			if !ok {
				t.Fatalf("tool %q is not registered", tt.toolName)
			}
			if routed := ai.Route(tt.text, []*ai.ToolMeta{tool}); len(routed) != 1 {
				t.Fatalf("Route(%q) = %v, want %q", tt.text, routed, tt.toolName)
			}
		})
	}
}

func TestHistoryToolRoutesTogetherWithContextualQQActions(t *testing.T) {
	history, ok := ai.GetTool("get_chat_history")
	if !ok {
		t.Fatal("get_chat_history is not registered")
	}

	for _, tc := range []struct {
		text       string
		actionName string
	}{
		{"禁言刚才说脏话的人", "ban_member"},
		{"撤回那条广告", "revoke_message"},
	} {
		action, ok := ai.GetTool(tc.actionName)
		if !ok {
			t.Fatalf("tool %q is not registered", tc.actionName)
		}
		routed := ai.Route(tc.text, []*ai.ToolMeta{history, action})
		if !containsRoutedTool(routed, "get_chat_history") || !containsRoutedTool(routed, tc.actionName) {
			t.Fatalf("Route(%q) = %v, want history and %s", tc.text, routed, tc.actionName)
		}
	}
}

func containsRoutedTool(results []ai.RouteResult, name string) bool {
	for _, result := range results {
		if result.Tool.Name == name {
			return true
		}
	}
	return false
}
