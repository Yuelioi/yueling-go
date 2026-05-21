package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
)

func init() {
	registerManageTodo()
	registerIPLookup()
	registerRandomPersona()
	registerDecodeAbbreviation()
	registerGetInspiration()
	registerSearchMusic()
}

// ── 待办管理 ──────────────────────────────────────────────────────────────────

func registerManageTodo() {
	ai.Register(ai.ToolMeta{
		Name:        "manage_todo",
		Description: "管理个人待办清单，支持添加、查看、完成、删除",
		Tags:        []string{"工具", "效率"},
		Triggers:    []string{"待办", "todo", "提醒我", "记一下"},
		Slots:       []string{"待办清单", "任务管理"},
		Params: []ai.Param{
			{Name: "action", Type: "string", Description: "操作: add/list/done/remove", Required: true},
			{Name: "content", Type: "string", Description: "待办内容（add时必填）", Required: false},
			{Name: "item_id", Type: "integer", Description: "编号（done/remove时必填）", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			action := ctx.String("action")
			content := ctx.String("content")
			itemID := ctx.Int("item_id")

			switch action {
			case "add":
				if content == "" {
					return "请说明待办内容", nil
				}
				if err := db.AddTodo(ctx.UserID(), ctx.GroupID(), content); err != nil {
					return err.Error(), nil
				}
				return "已添加待办: " + content, nil
			case "list":
				items, err := db.GetTodos(ctx.UserID())
				if err != nil || len(items) == 0 {
					return "你没有待办事项", nil
				}
				var sb strings.Builder
				sb.WriteString("你的待办:\n")
				for i, it := range items {
					sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, it.Content))
				}
				return strings.TrimRight(sb.String(), "\n"), nil
			case "done":
				if itemID <= 0 {
					return "请指定待办编号", nil
				}
				items, err := db.GetTodos(ctx.UserID())
				if err != nil || int(itemID) > len(items) {
					return fmt.Sprintf("编号不存在（共 %d 条）", len(items)), nil
				}
				target := items[itemID-1]
				if err := db.DoneTodo(target.ID); err != nil {
					return "操作失败", nil
				}
				return fmt.Sprintf("已完成: %s", target.Content), nil
			case "remove":
				if itemID <= 0 {
					return "请指定待办编号", nil
				}
				items, err := db.GetTodos(ctx.UserID())
				if err != nil || int(itemID) > len(items) {
					return fmt.Sprintf("编号不存在（共 %d 条）", len(items)), nil
				}
				target := items[itemID-1]
				if err := db.DeleteTodo(target.ID); err != nil {
					return "操作失败", nil
				}
				return fmt.Sprintf("已删除: %s", target.Content), nil
			}
			return "未知操作，支持: add/list/done/remove", nil
		},
	})
}

// ── IP 查询 ───────────────────────────────────────────────────────────────────

func registerIPLookup() {
	ai.Register(ai.ToolMeta{
		Name:        "ip_lookup",
		Description: "查询 IP 地址归属地",
		Tags:        []string{"工具", "信息"},
		Triggers:    []string{"IP", "ip", "归属"},
		Slots:       []string{"IP查询", "归属地"},
		Params: []ai.Param{
			{Name: "ip", Type: "string", Description: "IP地址", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			ip := strings.TrimSpace(ctx.String("ip"))
			resp, err := httpClient.Get(fmt.Sprintf("https://whois.pconline.com.cn/ipJson.jsp?ip=%s&json=true", ip))
			if err != nil {
				return "查询失败", nil
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var data struct {
				Addr string `json:"addr"`
				Pro  string `json:"pro"`
				City string `json:"city"`
			}
			json.Unmarshal(body, &data)
			addr := strings.TrimSpace(data.Addr)
			if addr == "" {
				addr = strings.TrimSpace(data.Pro + " " + data.City)
			}
			if addr == "" {
				return fmt.Sprintf("IP %s 归属地未知", ip), nil
			}
			return fmt.Sprintf("IP %s → %s", ip, addr), nil
		},
	})
}

// ── 随机人设 ──────────────────────────────────────────────────────────────────

func registerRandomPersona() {
	names := []string{"苍月", "白夜", "赤羽", "千织", "幽兰", "星河", "雪见", "墨染", "清浅", "夜阑"}
	races := []string{"人类", "精灵", "龙族后裔", "半神", "机械生命", "吸血鬼", "狐妖", "天使", "恶魔", "人鱼"}
	jobs := []string{"剑士", "法师", "盗贼", "吟游诗人", "炼金术士", "赏金猎人", "占星师", "驯兽师", "咒术师", "铸甲师"}
	traits := []string{"沉默寡言但内心温柔", "话痨且社牛", "傲娇毒舌", "天然呆", "中二病晚期", "老好人", "腹黑", "怕生但战斗力爆表", "吃货", "路痴"}
	weapons := []string{"双刃剑", "魔法书", "匕首", "竖琴", "炼金壶", "双枪", "星盘", "鞭子", "咒符", "盾牌"}
	likes := []string{"猫", "甜食", "下雨天", "古书", "星空", "酒", "睡觉", "打架", "收集宝石", "旅行"}

	ai.Register(ai.ToolMeta{
		Name:        "random_persona",
		Description: "生成一个随机虚拟角色人设",
		Tags:        []string{"娱乐", "创作"},
		Triggers:    []string{"人设", "随机人设"},
		Slots:       []string{"随机人设", "角色生成"},
		Params:      []ai.Param{},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			return fmt.Sprintf(
				"姓名: %s\n种族: %s\n职业: %s\n性格: %s\n武器: %s\n喜好: %s\n战力: %d",
				names[rand.Intn(len(names))],
				races[rand.Intn(len(races))],
				jobs[rand.Intn(len(jobs))],
				traits[rand.Intn(len(traits))],
				weapons[rand.Intn(len(weapons))],
				likes[rand.Intn(len(likes))],
				rand.Intn(99000)+1000,
			), nil
		},
	})
}

// ── 缩写解码 ──────────────────────────────────────────────────────────────────

func registerDecodeAbbreviation() {
	ai.Register(ai.ToolMeta{
		Name:        "decode_abbreviation",
		Description: "解码拼音缩写/网络黑话，如 yyds、xswl、awsl",
		Tags:        []string{"语言", "信息"},
		Triggers:    []string{"缩写", "什么意思", "啥意思"},
		Patterns:    []string{`\w{2,6}(是什么|啥意思)`},
		Slots:       []string{"网络黑话", "拼音缩写"},
		Params: []ai.Param{
			{Name: "text", Type: "string", Description: "要解码的缩写", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			text := strings.ToLower(strings.TrimSpace(ctx.String("text")))
			body, _ := json.Marshal(map[string]string{"text": text})
			resp, err := httpClient.Post(
				"https://lab.magiconch.com/api/nbnhhsh/guess",
				"application/json",
				bytes.NewReader(body),
			)
			if err != nil {
				return "查询失败", nil
			}
			defer resp.Body.Close()
			raw, _ := io.ReadAll(resp.Body)

			var data []struct {
				Name      string   `json:"name"`
				Trans     []string `json:"trans"`
				Inputting []string `json:"inputting"`
			}
			if err := json.Unmarshal(raw, &data); err != nil {
				return "查询失败", nil
			}
			var lines []string
			for _, item := range data {
				trans := item.Trans
				if len(trans) == 0 {
					trans = item.Inputting
				}
				if len(trans) > 5 {
					trans = trans[:5]
				}
				if len(trans) > 0 {
					lines = append(lines, fmt.Sprintf("%s: %s", item.Name, strings.Join(trans, "、")))
				}
			}
			if len(lines) == 0 {
				return fmt.Sprintf("没有找到「%s」的含义", text), nil
			}
			return strings.Join(lines, "\n"), nil
		},
	})
}

// ── 搜歌 ─────────────────────────────────────────────────────────────────────

func registerSearchMusic() {
	ai.Register(ai.ToolMeta{
		Name:        "search_music",
		Description: "搜索音乐，返回歌曲信息和链接",
		Tags:        []string{"娱乐", "音乐"},
		Triggers:    []string{"点歌", "歌"},
		Patterns:    []string{`来首.+`},
		Slots:       []string{"搜歌", "音乐搜索", "听歌"},
		Params: []ai.Param{
			{Name: "keyword", Type: "string", Description: "歌曲名或歌手名", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			kw := strings.TrimSpace(ctx.String("keyword"))
			resp, err := httpClient.Get(fmt.Sprintf("https://api.vvhan.com/api/music/wy?kw=%s&type=json", kw))
			if err != nil {
				return "搜索失败", nil
			}
			defer resp.Body.Close()
			var data struct {
				Success bool `json:"success"`
				Info    struct {
					Name   string `json:"name"`
					Author string `json:"auther"`
					URL    string `json:"url"`
				} `json:"info"`
			}
			raw, _ := io.ReadAll(resp.Body)
			if json.Unmarshal(raw, &data) != nil || !data.Success {
				return fmt.Sprintf("没找到「%s」相关的歌曲", kw), nil
			}
			result := fmt.Sprintf("%s - %s", data.Info.Name, data.Info.Author)
			if data.Info.URL != "" {
				result += "\n" + data.Info.URL
			}
			return result, nil
		},
	})
}

// ── 古诗词/鸡汤 ───────────────────────────────────────────────────────────────

func registerGetInspiration() {
	ai.Register(ai.ToolMeta{
		Name:        "get_inspiration",
		Description: "获取随机古诗词或心灵鸡汤",
		Tags:        []string{"娱乐", "文学"},
		Triggers:    []string{"诗", "鸡汤", "古诗"},
		Patterns:    []string{`来(首|碗|条).+`},
		Slots:       []string{"古诗词", "心灵鸡汤"},
		Params: []ai.Param{
			{Name: "category", Type: "string", Description: "poetry(古诗词) / soup(鸡汤) / comment(网易云热评)", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			category := ctx.String("category")
			if category == "poetry" || category == "" {
				resp, err := httpClient.Get("https://v1.jinrishici.com/all.json")
				if err == nil && resp.StatusCode == 200 {
					defer resp.Body.Close()
					var d struct {
						Content string `json:"content"`
						Author  string `json:"author"`
						Origin  string `json:"origin"`
					}
					if raw, _ := io.ReadAll(resp.Body); json.Unmarshal(raw, &d) == nil && d.Content != "" {
						return fmt.Sprintf("「%s」\n—— %s《%s》", d.Content, d.Author, d.Origin), nil
					}
				}
			}
			// soup and comment both use vvhan; comment uses the same endpoint
			resp, err := httpClient.Get("https://api.vvhan.com/api/ian/rand?type=json")
			if err != nil {
				return "获取失败", nil
			}
			defer resp.Body.Close()
			var d struct {
				Success bool `json:"success"`
				Data    struct {
					Content string `json:"content"`
				} `json:"data"`
			}
			if raw, _ := io.ReadAll(resp.Body); json.Unmarshal(raw, &d) == nil && d.Success {
				return d.Data.Content, nil
			}
			return "获取失败", nil
		},
	})
}
