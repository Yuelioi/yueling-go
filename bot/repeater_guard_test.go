package bot

import "testing"

// 复读插件「命令不复读」依赖 dispatcher 把 Command/FullMatch 命中标记为 commandMatched。
// 此测试守护分类：命令型 matcher 算命令（pack / 我老婆呢），被动/泛匹配不算
// （否则 Keyword/Any 会把所有消息都标成命令，复读彻底失效）。
func TestIsCommandMatcher(t *testing.T) {
	cases := []struct {
		name    string
		matcher Matcher
		want    bool
	}{
		{"Command(pack)", Command("pack"), true},
		{"FullMatch(我老婆呢/老婆)", FullMatch("我老婆呢", "老婆"), true},
		{"Keyword", Keyword("哈哈"), false},
		{"Regex", Regex("https?://"), false},
		{"Any", Any, false},
	}
	for _, c := range cases {
		if got := isCommandMatcher(c.matcher); got != c.want {
			t.Errorf("%s: isCommandMatcher = %v, want %v", c.name, got, c.want)
		}
	}
}

// commandMatched 默认 false（普通聊天可复读），CommandMatched() 如实反映。
func TestMsgCtxCommandMatchedDefault(t *testing.T) {
	m := &MsgCtx{}
	if m.CommandMatched() {
		t.Fatal("新建 MsgCtx 不应标记为命令")
	}
	m.commandMatched = true
	if !m.CommandMatched() {
		t.Fatal("置位后 CommandMatched() 应为 true")
	}
}
