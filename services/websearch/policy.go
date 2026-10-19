package websearch

import (
	"regexp"
	"strings"
)

var pageURL = regexp.MustCompile(`https?://[^\s<>"'，。！？；）】]+`)
var noSearch = regexp.MustCompile(`(不要|别|不用|不必|不需要|无需).{0,3}(联网|搜索|上网|搜|查资料)|\b(no search|do not search|don't search|without searching|don't browse|do not browse)\b`)

func urls(text string) []string {
	urls := pageURL.FindAllString(text, -1)
	for i := range urls {
		urls[i] = strings.TrimRight(urls[i], ").,;!?]}")
	}
	return urls
}

func SearchAllowed(text string) bool {
	text = strings.ToLower(pageURL.ReplaceAllString(text, ""))
	if noSearch.MatchString(text) {
		return false
	}
	intent := text
	for _, noun := range []string{"搜索引擎", "搜索算法", "搜索功能", "搜索工具", "二分搜索", "深度优先搜索", "广度优先搜索"} {
		intent = strings.ReplaceAll(intent, noun, "")
	}
	explicit := firstOf(intent, []string{"搜索", "搜一下", "搜一搜", "搜搜", "搜下", "帮我搜", "上网查", "网上查", "网上搜", "网上找", "联网", "查一下", "查下", "查一查", "帮我查", "查查", "查找", "查询", "检索", "查证", "核实", "求证", "找资料", "找一下", "查资料", "给个来源", "提供出处", "继续查", "再查", "继续搜", "再搜", "换个关键词", "是真的吗", "是否属实", "真的假的", "靠谱吗", "search for", "look up", "google", "browse"})
	transform := firstOf(text, []string{"翻译", "润色", "改写", "总结这段", "总结下面", "整理这段", "只根据", "仅根据", "translate", "rewrite"})
	if transform >= 0 && (explicit < 0 || transform < explicit) && firstOf(text, []string{"并搜", "并查", "再搜", "再查", "然后搜", "然后查", "顺便搜", "顺便查", "同时搜", "同时查"}) < 0 {
		return false
	}
	if explicit >= 0 {
		return true
	}
	for _, word := range []string{"最新", "现任", "实时", "近况", "新进展", "新闻", "时事", "current version", "latest", "recent news", "up to date"} {
		if strings.Contains(text, word) {
			return true
		}
	}
	if firstOf(text, []string{"是什么", "什么意思", "原理", "定义", "概念", "计算公式"}) < 0 {
		if firstOf(text, []string{"股价", "币价", "汇率", "票价", "余票", "天气预报"}) >= 0 {
			return true
		}
		if strings.Contains(text, "到") && firstOf(text, []string{"机票", "航班", "火车票", "高铁"}) >= 0 {
			return true
		}
	}
	if firstOf(text, []string{"最近", "近期", "目前", "现在", "刚刚", "今日", "今天", "今年"}) < 0 {
		return false
	}
	if firstOf(text, []string{"现在帮我解释", "现在解释", "现在讲讲", "现在说说"}) >= 0 {
		return false
	}
	return firstOf(text, []string{"什么", "啥", "谁", "哪", "吗", "是否", "如何", "怎么", "多少", "支持", "发布", "版本", "消息", "进展", "政策", "价格", "票价", "余票", "天气", "发生", "变化", "动态", "资讯", "情况"}) >= 0
}

func firstOf(text string, words []string) int {
	first := -1
	for _, word := range words {
		if i := strings.Index(text, word); i >= 0 && (first < 0 || i < first) {
			first = i
		}
	}
	return first
}

func PageReadAllowed(text, quoted, target string, searched bool) bool {
	for _, phrase := range []string{"不要联网", "不用联网", "别联网", "不要上网", "别上网", "不联网", "don't browse", "do not browse"} {
		if strings.Contains(strings.ToLower(text), phrase) {
			return false
		}
	}
	if searched {
		return true
	}
	for _, raw := range urls(text) {
		if raw == target {
			return true
		}
	}
	read := false
	for _, word := range []string{"解释", "解读", "总结", "分析", "看看", "看下", "读一下", "讲讲", "说什么", "重点", "这篇", "这个链接", "这条链接", "read", "summarize", "explain"} {
		read = read || strings.Contains(strings.ToLower(text), word)
	}
	if read {
		for _, raw := range urls(quoted) {
			if raw == target {
				return true
			}
		}
	}
	return SearchAllowed(text)
}
