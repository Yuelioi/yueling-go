package tools

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

func RegisterSearchAE(b *bot.Bot) {
	b.OnCommand("搜ae插件", "搜ae脚本").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply("用法：搜ae插件 关键词")
		}
		ctx.React(bot.EmojiProcessing)
		kw := strings.Join(ctx.Args, " ")
		result, err := searchLookAE(kw)
		if err != nil || result == "" {
			return ctx.Reply(fmt.Sprintf("搜索不到，请访问原网站 https://www.lookae.com/?s=%s", url.QueryEscape(kw)))
		}
		return ctx.Reply(result)
	})
}

type aeArticle struct {
	title   string
	link    string
	baiduDL string
}

func searchLookAE(kw string) (string, error) {
	searchURL := "https://www.lookae.com/?s=" + url.QueryEscape(kw)
	body, err := httpclient.Direct.GetBytes(searchURL)
	if err != nil {
		return "", err
	}

	articles := parseLookAEArticles(body)
	if len(articles) == 0 {
		return "", nil
	}
	if len(articles) > 3 {
		articles = articles[:3]
	}

	// Fetch baidu pan links for each article concurrently (simple sequential)
	for i := range articles {
		if dl := fetchBaiduLink(articles[i].link); dl != "" {
			articles[i].baiduDL = dl
		}
	}

	var sb strings.Builder
	for i, a := range articles {
		fmt.Fprintf(&sb, "%d.%s\n网站: %s\n", i+1, a.title, a.link)
		if a.baiduDL != "" {
			fmt.Fprintf(&sb, "百度网盘: %s\n", a.baiduDL)
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String()), nil
}

func parseLookAEArticles(body []byte) []aeArticle {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	var articles []aeArticle
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "article" {
			var title, link string
			var findH2 func(*html.Node)
			findH2 = func(c *html.Node) {
				if c.Type == html.ElementNode && c.Data == "h2" {
					var findA func(*html.Node)
					findA = func(a *html.Node) {
						if a.Type == html.ElementNode && a.Data == "a" {
							for _, attr := range a.Attr {
								if attr.Key == "href" {
									link = attr.Val
								}
							}
							if a.FirstChild != nil {
								title = strings.TrimSpace(a.FirstChild.Data)
							}
						}
						for child := a.FirstChild; child != nil; child = child.NextSibling {
							findA(child)
						}
					}
					findA(c)
				}
				for child := c.FirstChild; child != nil; child = child.NextSibling {
					findH2(child)
				}
			}
			findH2(n)
			if title != "" && link != "" {
				articles = append(articles, aeArticle{title: title, link: link})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return articles
}

func fetchBaiduLink(pageURL string) string {
	body, err := httpclient.Direct.GetBytes(pageURL)
	if err != nil {
		return ""
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return ""
	}
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			var href, text string
			for _, a := range n.Attr {
				if a.Key == "href" {
					href = a.Val
				}
			}
			if n.FirstChild != nil {
				text = n.FirstChild.Data
			}
			if strings.Contains(text, "百度网盘") && strings.Contains(href, "pan.baidu.com") {
				found = href
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}
