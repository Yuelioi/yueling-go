package system

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
)

type pluginEntry struct {
	ID       int
	Name     string
	Group    string
	Desc     string
	Usage    string
	Commands []string
}

var pluginRegistry = []pluginEntry{
	// ── 群管 ──────────────────────────────────────────────────────────────
	{1, "禁言管理", "群管",
		"禁言、解禁、踢人、撤回、全员禁言",
		"  禁言 @用户 [时长]    时长格式：10m / 1h / 600（秒），默认 10 分钟\n" +
			"  解禁 @用户\n" +
			"  踢出 @用户\n" +
			"  撤回             回复目标消息后发送\n" +
			"  全员禁言 / 全员禁言 off",
		[]string{"禁言", "解禁", "踢出", "撤回", "全员禁言"}},

	{2, "关键词回复", "群管",
		"设置关键词触发自动回复",
		"  关键词 <触发词> <回复内容>    添加\n" +
			"  查看关键词                    列出所有\n" +
			"  删除关键词 <触发词>",
		[]string{"关键词", "查看关键词", "删除关键词"}},

	{3, "入群审批", "群管",
		"含关键词的入群申请自动通过",
		"  在 config.toml 中设置 join_keywords 列表\n" +
			"  申请消息含关键词 → 自动同意，否则拒绝",
		[]string{}},

	{4, "群友备份", "群管",
		"将群成员信息备份为 JSON 文件",
		"  群友备份    导出当前群成员列表到 data/members/{groupID}.json",
		[]string{"群友备份"}},

	{5, "群规则", "群管",
		"管理群规则条目",
		"  群规则                列出所有规则\n" +
			"  添加群规则 <内容>     追加一条\n" +
			"  删除群规则 <ID>       按序号删除",
		[]string{"群规则", "添加群规则", "删除群规则"}},

	{6, "自动回复", "群管",
		"管理精确触发的自动回复词条",
		"  添加回复 <触发词> <回复内容>\n" +
			"  删除回复 <触发词>\n" +
			"  查看回复",
		[]string{"添加回复", "删除回复", "查看回复"}},

	{7, "群文件", "群管",
		"群文件备份、恢复、整理、清理、本地清理、查询",
		"  群文件备份              扫描所有子文件夹 → 下载缺失文件到本地\n" +
			"  群文件恢复              本地 → 上传缺失文件到群（自动建文件夹）\n" +
			"  群文件清理 [扩展名...]  删除指定扩展名文件（默认：gif png jpg mp4 等）\n" +
			"  群文件整理 <文件夹> <扩展名...>  将根目录文件移入指定文件夹\n" +
			"  本地文件清理            删除本地备份目录\n" +
			"  群文件查询 <关键词>     模糊搜索文件名（最多5条）",
		[]string{"群文件备份", "群文件恢复", "群文件清理", "群文件整理", "本地文件清理", "群文件查询"}},

	{8, "用户标签", "群管",
		"为自己设置键值对标签，AI 可自动读取（如位置用于查天气）",
		"  添加标签 <键> <值>    如：添加标签 位置 上海\n" +
			"  删除标签 <键>\n" +
			"  我的标签              查看全部标签",
		[]string{"添加标签", "删除标签", "我的标签"}},

	// ── 游戏 ──────────────────────────────────────────────────────────────
	{9, "签到系统", "游戏",
		"每日签到、积分查询、群排行",
		"  签到\n" +
			"  积分\n" +
			"  排行",
		[]string{"签到", "积分", "排行"}},

	{10, "PK对战", "游戏",
		"和群友随机 PK，胜负影响积分",
		"  pk @用户",
		[]string{"pk"}},

	{11, "剑网三物价", "游戏",
		"查询剑网三游戏内物品价格",
		"  物价 <关键词>",
		[]string{"物价"}},

	{12, "打卡", "游戏",
		"每日打卡，自动将群名片末尾数字 +1（如 小明123 → 小明124）",
		"  打卡",
		[]string{"打卡"}},

	// ── 提醒 ──────────────────────────────────────────────────────────────
	{13, "定时提醒", "提醒",
		"每日定时或一次性提醒",
		"  提醒 HH:MM <内容>       每天该时间提醒\n" +
			"  提醒 N分钟后 <内容>     一次性，N 分钟后触发\n" +
			"  提醒 N小时后 <内容>     一次性，N 小时后触发\n" +
			"  我的提醒               查看所有提醒\n" +
			"  取消提醒 <ID>",
		[]string{"提醒", "我的提醒", "取消提醒"}},

	// ── 随机 ──────────────────────────────────────────────────────────────
	{14, "随机群友", "随机",
		"随机 @ 一位群友",
		"  抽群友 / 来个群友",
		[]string{"抽群友", "来个群友"}},

	{15, "随机取名", "随机",
		"随机更换自己的群名片",
		"  随机取名",
		[]string{"随机取名"}},

	{16, "骰子", "随机",
		"各种 roll 模式",
		"  roll              1-100\n" +
			"  roll N            1-N\n" +
			"  roll N M          N-M\n" +
			"  roll A B C ...    从列表随机选",
		[]string{"roll"}},

	{17, "表情包", "随机",
		"前置空格触发本地表情包",
		"  <空格>关键词      随机发一张匹配的表情包\n" +
			"  <两个空格>关键词  列出匹配文件名",
		[]string{}},

	{18, "随机图片", "随机",
		"发送本地素材库中的随机图片",
		"  龙图 / 福瑞 / 老婆 / 老公 / 沙雕图 / 杂鱼 / 美少女\n" +
			"  随机猫猫 / 来点猫猫\n" +
			"  语录 [名字]    群友语录，可按名字筛选",
		[]string{"龙图", "福瑞", "老婆", "老公", "沙雕图", "杂鱼", "美少女", "随机猫猫", "语录"}},

	{19, "日常随机", "随机",
		"随机推荐吃喝玩乐，发 2×2 图片网格",
		"  随机喝的 / 喝啥 / 喝什么 / 来点喝的\n" +
			"  随机吃的 / 吃啥 / 吃什么 / 来点吃的\n" +
			"  随机玩的 / 玩啥 / 玩什么 / 来点玩的\n" +
			"  随机水果 / 来点水果",
		[]string{"随机喝的", "随机吃的", "随机玩的", "随机水果"}},

	// ── 娱乐 ──────────────────────────────────────────────────────────────
	{20, "今日运势", "娱乐",
		"每人每天抽一次签，生成运势图片",
		"  今日运势 / 运势 / 求签\n" +
			"  运势 <主题>          指定主题，如：运势 ba\n" +
			"  可用主题目录见 data/fortune/themes/",
		[]string{"今日运势", "运势", "求签"}},

	{21, "热搜", "娱乐",
		"查看各平台实时热搜榜",
		"  热搜 / 查热搜          显示微博、B站、百度、抖音热搜",
		[]string{"热搜", "查热搜"}},

	{22, "好感度", "娱乐",
		"查看与月灵的好感度（基于积分）",
		"  查看好感度",
		[]string{"查看好感度"}},

	{23, "睡觉", "娱乐",
		"禁言自己 5~8 小时（强制休息）",
		"  我要睡觉",
		[]string{"我要睡觉"}},

	{24, "戳一戳", "娱乐",
		"戳月灵触发随机回应",
		"  直接戳（被动触发）",
		[]string{}},

	{25, "场景识别", "娱乐",
		"识别图片中的动漫场景（需代理）",
		"  场景识别 + 图片    返回动漫名、集数、时间点、相似度",
		[]string{"场景识别"}},

	// ── 工具 ──────────────────────────────────────────────────────────────
	{26, "翻译", "工具",
		"多语言互译",
		"  翻译 <文字>       AI 判断语言自动翻译\n" +
			"  中译英 / 英译中 / 中译日 / 日译中 / 英译日 / 日译英\n" +
			"  用法：中译英 你好",
		[]string{"翻译", "中译英", "英译中", "中译日", "日译中", "英译日", "日译英"}},

	{27, "搜AE插件", "工具",
		"在 lookae.com 搜索 After Effects 插件/脚本",
		"  搜ae插件 <关键词>\n" +
			"  搜ae脚本 <关键词>",
		[]string{"搜ae插件", "搜ae脚本"}},

	{28, "链接解析", "工具",
		"消息含链接时自动解析预览（被动触发）",
		"  支持平台：\n" +
			"    B站  视频(BV/av) / 番剧(ep/ss) / 直播 / b23.tv 短链\n" +
			"    知乎  zhuanlan.zhihu.com 专栏\n" +
			"    CSDN  blog.csdn.net\n" +
			"    微博  weibo.com\n" +
			"    Twitter/X  x.com（需代理）\n" +
			"    Behance  behance.net（需代理）",
		[]string{}},

	// ── AI ────────────────────────────────────────────────────────────────
	{29, "AI 助手", "AI",
		"@月灵 或消息以「月灵」开头触发，支持多轮对话",
		"  @月灵 <问题>\n" +
			"  月灵 <问题>\n\n" +
			"  内置工具（AI 自动选用）：\n" +
			"    天气 / 翻译 / 搜索 / 机票 / 火车票\n" +
			"    星座运势 / 老黄历 / 成语接龙\n" +
			"    IP查询 / 待办管理 / 点歌 / 汇率\n" +
			"    代码助手 / 文本摘要 / 日期计算\n" +
			"    聊天记录总结 / 查群成员 / 匿名传话\n" +
			"    禁言/踢出（需管理权限，二次确认）",
		[]string{}},

	// ── 系统 ──────────────────────────────────────────────────────────────
	{30, "帮助", "系统",
		"插件帮助系统",
		"  帮助 / help               插件清单\n" +
			"  帮助 <ID>               按 ID 查看详细用法\n" +
			"  帮助 <插件名>           按名称查找\n" +
			"  帮助 <分组名>           列出该分组所有插件",
		[]string{"帮助", "help"}},

	{31, "系统工具", "系统",
		"连通测试与重启",
		"  ping       连通测试\n" +
			"  重启       重启 bot（超管专用）",
		[]string{"ping", "重启"}},
}

// ── Index ─────────────────────────────────────────────────────────────────────

var (
	pluginByID    = map[int]*pluginEntry{}
	pluginByName  = map[string]*pluginEntry{}
	pluginByCmd   = map[string]*pluginEntry{}
	pluginGroups  = map[string][]*pluginEntry{}
)

func init() {
	for i := range pluginRegistry {
		p := &pluginRegistry[i]
		pluginByID[p.ID] = p
		pluginByName[strings.ToLower(p.Name)] = p
		for _, cmd := range p.Commands {
			pluginByCmd[strings.ToLower(cmd)] = p
		}
		pluginGroups[p.Group] = append(pluginGroups[p.Group], p)
	}
}

// ── Formatters ────────────────────────────────────────────────────────────────

var groupOrder = []string{"群管", "游戏", "提醒", "随机", "娱乐", "工具", "AI", "系统"}

func formatList() string {
	var sb strings.Builder
	sb.WriteString("月灵插件清单\n帮助 <ID/名称/分组> 查看详细用法\n")
	for _, grp := range groupOrder {
		entries := pluginGroups[grp]
		if len(entries) == 0 {
			continue
		}
		sb.WriteString("\n【" + grp + "】\n")
		for _, p := range entries {
			sb.WriteString(fmt.Sprintf("  #%-2d %s — %s\n", p.ID, p.Name, p.Desc))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatDetail(p *pluginEntry) string {
	var sb strings.Builder
	sb.WriteString("━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("分组：%s\n名称：%s（#%d）\n", p.Group, p.Name, p.ID))
	if len(p.Commands) > 0 {
		sb.WriteString("命令：" + strings.Join(p.Commands, " / ") + "\n")
	}
	sb.WriteString("━━━━━━━━━━━━━━━━\n")
	sb.WriteString("用法：\n" + p.Usage + "\n")
	sb.WriteString("━━━━━━━━━━━━━━━━")
	return sb.String()
}

func formatGroup(grpName string) string {
	entries := pluginGroups[grpName]
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("【" + grpName + "】帮助 <ID/名称> 查看详细用法\n")
	for _, p := range entries {
		sb.WriteString(fmt.Sprintf("  #%-2d %s — %s\n", p.ID, p.Name, p.Desc))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// ── Search ─────────────────────────────────────────────────────────────────────

func helpSearch(query string) (string, bool) {
	q := strings.TrimSpace(query)

	// by ID
	if id, err := strconv.Atoi(q); err == nil {
		if p := pluginByID[id]; p != nil {
			return formatDetail(p), true
		}
		return "未找到 ID 为 " + q + " 的插件", false
	}

	// by name
	if p := pluginByName[strings.ToLower(q)]; p != nil {
		return formatDetail(p), true
	}

	// by command
	if p := pluginByCmd[strings.ToLower(q)]; p != nil {
		return formatDetail(p), true
	}

	// by group
	if grp := formatGroup(q); grp != "" {
		return grp, true
	}

	return "未找到插件「" + q + "」，试试 帮助 查看完整清单", false
}

// ── Register ───────────────────────────────────────────────────────────────────

func RegisterHelp(b *bot.Bot) {
	b.OnCommand("help", "帮助").Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			return ctx.Reply(formatList())
		}
		query := strings.Join(ctx.Args, " ")
		msg, _ := helpSearch(query)
		return ctx.Reply(msg)
	})
}
