package websearch

import "testing"

func TestSearchIntentIsConservative(t *testing.T) {
	for _, tc := range []struct {
		text    string
		allowed bool
	}{
		{"你好", false}, {"量子纠缠是什么", false}, {"什么是搜索引擎", false}, {"解释二分搜索算法", false},
		{"今天心情很好", false}, {"最近很累", false}, {"现在帮我解释什么是量子纠缠", false},
		{"股价是什么意思", false}, {"汇率计算公式", false}, {"上海到北京机票多少钱", true}, {"苹果股价", true},
		{"帮我翻译这句话：Search for latest news", false}, {"翻译这个词：搜索", false}, {"帮我翻译并搜索相关背景", true},
		{"解释这张图片", false}, {"帮我润色：今天发布最新消息", false}, {"翻译这句 recent news", false},
		{"不要搜索，解释最新的这段话", false}, {"别搜，说一下目前版本", false}, {"don't search the latest news", false},
		{"介绍 https://example.com/search/latest", false},
		{"帮我搜一下量子纠缠", true}, {"查查DeepSeek支持什么", true}, {"帮我找一下官方资料", true},
		{"DeepSeek最新有什么功能", true}, {"现在支持图片吗", true}, {"这条消息是真的吗", true}, {"目前的政策是什么", true},
		{"继续查", true}, {"Search for official sources", true},
	} {
		if got := SearchAllowed(tc.text); got != tc.allowed {
			t.Errorf("%q = %v", tc.text, got)
		}
	}
}

func TestPageReadingRequiresRelevantRequest(t *testing.T) {
	const url = "https://example.com/a"
	for _, tc := range []struct {
		text, quoted      string
		searched, allowed bool
	}{
		{"谢谢", "", false, false}, {"解释这篇文章", url, false, true}, {"你好", url, false, false},
		{"总结 " + url, "", false, true}, {"继续查资料", "", false, true}, {"看看细节", "", true, true},
		{"不要联网解释这段话", url, true, false}, {"不用搜索，只读这个链接 " + url, "", false, true},
	} {
		if got := PageReadAllowed(tc.text, tc.quoted, url, tc.searched); got != tc.allowed {
			t.Errorf("%+v = %v", tc, got)
		}
	}
}
