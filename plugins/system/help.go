package system

import (
	"strconv"
	"strings"
	"sync"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/plugins/image"
	"github.com/Yuelioi/yueling-go/services/logx"
)

type pluginEntry struct {
	ID       int
	Name     string
	Group    string
	Desc     string
	Usage    string
	Commands []string
}

type PluginCatalogEntry struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	Desc     string   `json:"desc"`
	Usage    string   `json:"usage"`
	Commands []string `json:"commands"`
}

var pluginRegistry = []pluginEntry{
	// ── 群管 ──────────────────────────────────────────────────────────────
	{1, "禁言管理", "群管",
		"禁言、解禁、撤回、全员禁言",
		"  禁言 @用户 [时长]    时长格式：10m / 1h / 600（秒），默认 10 分钟\n" +
			"  解禁 @用户\n" +
			"  撤回             回复目标消息后发送\n" +
			"  全员禁言 / 全员禁言 off",
		[]string{"禁言", "解禁", "撤回", "全员禁言"}},

	{3, "加群审核", "群管",
		"每群独立配置：加群申请理由命中关键词自动通过 / 拒绝（拒绝优先，其余留人工）",
		"  加群审核              查看本群白名单 / 黑名单\n" +
			"  加群白名单 词1,词2     覆盖通过词（可逗号多词；留空清空；填 * 任意理由放行）\n" +
			"  加群黑名单 词1,词2     覆盖拒绝词（留空清空）",
		[]string{"加群审核", "加群白名单", "加群黑名单"}},

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

	{7, "群文件", "群管",
		"群文件备份、恢复、整理、清理、本地清理、查询",
		"  群文件备份              扫描所有子文件夹 → 下载缺失文件到本地\n" +
			"  群文件恢复              本地 → 上传缺失文件到群（自动建文件夹）\n" +
			"  群文件清理 [扩展名...]  删除指定扩展名文件（默认：gif png jpg mp4 等）\n" +
			"  群文件整理 <文件夹> <扩展名...>  将根目录文件移入指定文件夹\n" +
			"  本地文件清理            删除本地备份目录\n" +
			"  群文件查询 <关键词>     模糊搜索文件名（最多5条）",
		[]string{"群文件备份", "群文件恢复", "群文件清理", "群文件整理", "本地文件清理", "群文件查询"}},

	{8, "个人资料与记忆", "AI",
		"用自然语言维护称呼、位置、时区等资料，以及可查看、可删除的长期记忆",
		"  @月灵 以后叫我阿七\n" +
			"  @月灵 记住我不吃香菜\n" +
			"  @月灵 你记得我什么 / 忘掉记忆 12",
		[]string{"我的资料", "记住我", "查看记忆", "忘掉记忆"}},

	{36, "设精", "群管",
		"把一条消息加入群精华（群管理员可用，需 bot 是群管理员）",
		"  设精 / 加精    回复目标消息后发送",
		[]string{"设精", "加精"}},

	// ── 游戏 ──────────────────────────────────────────────────────────────
	{9, "签到系统", "游戏",
		"每日签到、积分查询、群排行",
		"  签到\n" +
			"  积分\n" +
			"  排行",
		[]string{"签到", "积分", "排行"}},

	{11, "剑网三物价", "游戏",
		"查询剑网三游戏内物品价格",
		"  物价 <关键词>",
		[]string{"物价"}},

	{12, "打卡", "游戏",
		"每日打卡，群名片末尾数字 +1，并按本月次数随机送一句鼓励",
		"  打卡",
		[]string{"打卡"}},

	{41, "游戏折扣", "游戏",
		"查询 Epic 本周限免，以及 Steam 国区当前价和参考史低，不调用 AI",
		"  限免 / Epic限免 / 喜加一       Epic 本周免费游戏和下期预告\n" +
			"  史低 <游戏名 / appid / 链接>  Steam 当前价格、折扣和参考史低\n" +
			"  查价 <游戏名>                 同上",
		[]string{"限免", "Epic限免", "喜加一", "史低", "查价", "Steam"}},

	// ── 提醒 ──────────────────────────────────────────────────────────────
	{13, "定时提醒", "提醒",
		"用自然语言创建、修改、顺延每日/工作日/每周/一次性提醒",
		"  @月灵 明天下午三点提醒我交材料\n" +
			"  @月灵 每个工作日九点提醒我打卡\n" +
			"  @月灵 把提醒 12 改到下周一\n" +
			"  我的提醒               查看所有提醒\n" +
			"  取消提醒 <ID>",
		[]string{"提醒我", "修改提醒", "推迟提醒", "我的提醒", "取消提醒"}},

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
			"  roll + GIF        从附件或引用 GIF 随机抽一帧\n" +
			"  roll N            1-N\n" +
			"  roll N M          N-M\n" +
			"  roll A B C ...    从列表随机选",
		[]string{"roll"}},

	{17, "表情包", "随机",
		"前置空格触发本地表情包",
		"  <空格>关键词      随机发一张匹配的表情包\n" +
			"  <两个空格>关键词  列出匹配文件名",
		[]string{}},

	// #18 随机图片：Usage/Commands 运行期由 image 配置表生成（见 finalizeRegistry）。
	// 含吃喝玩乐 4合1（原 #19 日常随机已并入此条）。
	{18, "随机图片", "随机",
		"发送本地素材库中的随机图片（含吃喝玩乐 4合1）",
		"",
		nil},

	// #32 素材上传：Usage/Commands 运行期由 image 配置表 + 语录/表情生成（见 finalizeRegistry）。
	{32, "素材上传", "随机",
		"上传图片到本地素材库（任何人可用，相同图片不重复收录）",
		"",
		nil},

	// ── 娱乐 ──────────────────────────────────────────────────────────────
	{20, "今日运势", "娱乐",
		"每人每天抽一次签，生成运势图片",
		"  今日运势 / 运势 / 抽签\n" +
			"  运势 <主题>          指定主题，如：运势 ba\n" +
			"  可用主题目录见 data/fortune/themes/",
		[]string{"今日运势", "运势", "抽签"}},

	{21, "热搜", "娱乐",
		"查看各平台实时热搜榜",
		"  热搜 / 查热搜          显示微博、B站、百度、抖音热搜",
		[]string{"热搜", "查热搜"}},

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

	{33, "表情包生成", "娱乐",
		"130+ 种表情包模板，头像/文字驱动（需 meme-generator-rs 服务）",
		"  头像表情包              查看所有头像类模板列表（图片）\n" +
			"  随机表情               随机挑一个模板，用自己头像生成\n" +
			"  随机表情 @某人         随机模板，1张图用对方头像，2张图自己+对方\n" +
			"  表情详情 <关键词>      查看指定模板的关键词/参数/预览图\n" +
			"  <关键词>               直接触发，如：摸摸 / 亲亲 / 字符画 文字\n" +
			"  <关键词> @某人         用对方头像生成\n" +
			"  <关键词> + 图片        用附图或引用消息中的图生成\n\n" +
			"  图片优先级：附图 > 引用消息图 > @用户头像 > 发送者头像\n" +
			"  无文字时自动使用模板默认文字",
		[]string{"头像表情包", "随机表情", "表情详情", "表情帮助", "表情示例"}},

	{43, "月灵自制表情包", "娱乐",
		"yueling-go 自有的纯 Go 头像表情模板，与外部 meme 服务独立",
		"  月灵表情包 / 自制表情包    查看自制模板列表\n" +
			"  单身 + 图片                 保留上半部分，替换下半部分图片\n" +
			"  暂时不恋爱 / 没有恋爱的打算  同上",
		[]string{"月灵表情包", "自制表情包", "单身", "单身打算", "暂时不恋爱", "没有恋爱的打算"}},

	// ── 工具 ──────────────────────────────────────────────────────────────
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

	{34, "图片打包", "工具",
		"把一条消息里的所有图片打包成 zip 传到群文件",
		"  pack    回复目标消息后发送（图多的消息 / 合并转发，自带图也可）\n" +
			"  递归展开合并转发，所有图进同一个 zip",
		[]string{"pack"}},

	{35, "zssm", "工具",
		"「这是什么」——用视觉模型解读图片内容",
		"  zssm + 图片    解读所附图片\n" +
			"  zssm          回复一条带图的消息后发送",
		[]string{"zssm"}},

	{42, "图片处理", "工具",
		"本地处理静图或 GIF，支持附件与引用图片",
		"  镜像 / 翻转 / 水平翻转    左右翻转整张图片\n" +
			"  上下翻转 / 垂直翻转      上下翻转整张图片\n" +
			"  左镜像 / 右镜像          保留指定半边并镜像\n" +
			"  上镜像 / 下镜像          保留指定半边并镜像\n" +
			"  旋转 [90|180|270|-90]    默认顺时针 90°\n" +
			"  缩放 50% / 512 / 512x512\n" +
			"  灰度 / 黑白              转为灰度图\n" +
			"  反色 / 负片              反转 RGB 颜色\n\n" +
			"  所有命令均可附图，或回复图片后发送；GIF 会逐帧处理",
		[]string{"镜像", "翻转", "水平翻转", "左右翻转", "上下翻转", "垂直翻转", "左镜像", "右镜像", "上镜像", "下镜像", "旋转", "缩放", "灰度", "黑白", "反色", "负片"}},

	// ── AI ────────────────────────────────────────────────────────────────
	{29, "AI 助手", "AI",
		"@月灵 或消息以「月灵」开头触发，支持多轮对话和可控长期记忆",
		"  @月灵 <问题>\n" +
			"  月灵 <问题>\n\n" +
			"  会话与记忆：\n" +
			"    月灵 新对话\n" +
			"    月灵 你记得我什么\n" +
			"    月灵 忘记第 N 条记忆\n" +
			"    月灵 清空我的长期记忆\n\n" +
			"  内置工具（AI 自动选用）：\n" +
			"    天气 / 搜索 / 机票 / 火车票\n" +
			"    星座运势 / 老黄历\n" +
			"    IP查询 / 真实提醒 / 待办管理 / 点歌 / 汇率\n" +
			"    聊天记录总结 / 群聊统计 / 查群成员\n" +
			"    个人资料与记忆 / 群规则 / 知识库与快捷词 / 群聊日报\n" +
			"    设精 / 改群名片 / 设头衔 / 戳一戳（QQ 动作）\n" +
			"    撤回（群管理员）/ 禁言（可联动聊天记录）",
		[]string{}},

	{37, "群聊日报", "AI",
		"每天定时总结群聊的主要话题、重要信息和待跟进事项",
		"  群聊日报 HH:MM [10-100]    开启或更新时间与消息条数（默认80）\n" +
			"  群聊日报 现在 [10-100]       立即生成一次，不修改定时设置\n" +
			"  群聊日报                    查看当前设置\n" +
			"  群聊日报 off                关闭（群管理员可用）",
		[]string{"群聊日报"}},

	{38, "订阅中心", "工具",
		"订阅 RSS/Atom 信息源，发现更新后自动推送到群聊",
		"  订阅添加 <URL> [名称]    添加订阅并从当前最新内容开始（群管理员）\n" +
			"  订阅B站视频 <UID/主页> [名称]  订阅 UP 主投稿（群管理员）\n" +
			"  订阅B站动态 <UID/主页> [名称]  订阅 UP 主动态（群管理员）\n" +
			"  订阅B站直播 <房间号/链接> [名称] 订阅开播（群管理员）\n" +
			"  订阅X <用户名/主页> [名称]      订阅用户发推（群管理员）\n" +
			"  订阅列表                 查看本群订阅\n" +
			"  订阅状态                 查看异常源、待推送数量和静默设置\n" +
			"  订阅暂停 <ID>            暂停单个订阅并清理其待推送内容（群管理员）\n" +
			"  订阅恢复 <ID>            恢复单个订阅（群管理员）\n" +
			"  订阅静默 23:00-08:00    夜间继续抓取、结束后合并推送（群管理员）\n" +
			"  订阅静默 off            关闭静默时段（群管理员）\n" +
			"  订阅检查                 立即检查；非静默时段同步推送（群管理员）\n" +
			"  订阅删除 <ID>            删除订阅（群管理员）\n" +
			"  每群最多 10 个，每 10 分钟自动检查；失败自动退避并持久重试",
		[]string{"订阅添加", "订阅B站视频", "订阅B站动态", "订阅B站直播", "订阅X", "订阅列表", "订阅状态", "订阅暂停", "订阅恢复", "订阅静默", "订阅检查", "订阅删除"}},

	{39, "群知识库", "AI",
		"一份资料同时支持有来源 AI 问答和零 AI 的精确快捷回复",
		"  知识添加 [标题] | <内容>   添加文本知识（群管理员）\n" +
			"  回复文字 + 知识添加 [标题]  保存被回复内容（群管理员）\n" +
			"  知识导入 <URL> [标题]      导入公网网页（群管理员）\n" +
			"  知识列表                   查看本群知识条目\n" +
			"  知识问 <问题>              仅根据本群资料回答并引用知识 ID\n" +
			"  知识删除 <ID>              删除知识（群管理员）\n" +
			"  @月灵 给知识 #12 设置快捷词 ae下载（群管理员）\n" +
			"  直接发送快捷词会精确命中并秒回，不调用 AI；也可在 WebUI 编辑",
		[]string{"知识添加", "知识导入", "知识列表", "知识问", "知识删除"}},

	{40, "群聊词云", "娱乐",
		"本地统计群聊热词、发言榜和个人口头禅，不调用 AI，记录仅保留 35 天",
		"  词云 / 今日词云       今日群聊词云\n" +
			"  昨日词云 / 本周词云  指定时间范围\n" +
			"  我的词云              自己今日的词云\n" +
			"  今日废话榜 / 本周龙王  群友消息数排行\n" +
			"  口头禅 [@群友]        最近七天高频用语\n" +
			"  谁最爱说 <关键词>     最近七天谁最常说这个词",
		[]string{"词云", "今日词云", "昨日词云", "本周词云", "我的词云", "废话榜", "今日废话榜", "本周废话榜", "今日龙王", "本周龙王", "口头禅", "谁最爱说"}},

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
	pluginByID   = map[int]*pluginEntry{}
	pluginByName = map[string]*pluginEntry{}
	pluginByCmd  = map[string]*pluginEntry{}
	pluginGroups = map[string][]*pluginEntry{}
)

var finalizeOnce sync.Once

func ensureRegistry() {
	finalizeOnce.Do(finalizeRegistry)
}

func Catalog() []PluginCatalogEntry {
	return buildCatalogEntries()
}

func buildCatalogEntries() []PluginCatalogEntry {
	out := make([]PluginCatalogEntry, 0, len(pluginRegistry))
	for _, entry := range pluginRegistry {
		commands := append([]string(nil), entry.Commands...)
		catalogEntry := PluginCatalogEntry{
			ID:       entry.ID,
			Name:     entry.Name,
			Group:    entry.Group,
			Desc:     entry.Desc,
			Usage:    entry.Usage,
			Commands: commands,
		}
		applyCatalogDynamicFields(&catalogEntry)
		if catalogEntry.Commands == nil {
			catalogEntry.Commands = []string{}
		}
		out = append(out, catalogEntry)
	}
	return out
}

func applyCatalogDynamicFields(entry *PluginCatalogEntry) {
	quotationCall, quotationAdd := quotationHelpSyntax()
	switch entry.ID {
	case 18: // 随机图片（single/grid/external 调用 + 语录）
		entry.Usage = image.HelpCallUsage() +
			"\n  " + quotationCall + "    群友语录，可按名字筛选"
		entry.Commands = append(image.HelpCallCommands(), "语录")
	case 32: // 素材上传（image 添加 + 表情/语录添加）
		entry.Usage = image.HelpAddUsage() +
			"\n  添加表情 [关键词] + 图片   按关键词索引，用于空格触发" +
			"\n  " + quotationAdd + "   + 图片   按群+昵称索引，语录命令可查" +
			"\n  支持同时上传多张；引用含图片的消息也可触发"
		entry.Commands = append(image.HelpAddCommands(), "添加表情", "添加语录")
	}
}

// finalizeRegistry 填充图片相关条目的动态字段（依赖 image.Register 已设置
// activeEntries），再构建索引。必须在 image.Register 之后调用（见 RegisterHelp）。
func finalizeRegistry() {
	quotationCall, quotationAdd := quotationHelpSyntax()
	for i := range pluginRegistry {
		switch pluginRegistry[i].ID {
		case 18: // 随机图片（single/grid/external 调用 + 语录）
			pluginRegistry[i].Usage = image.HelpCallUsage() +
				"\n  " + quotationCall + "    群友语录，可按名字筛选"
			pluginRegistry[i].Commands = append(image.HelpCallCommands(), "语录")
		case 32: // 素材上传（image 添加 + 表情/语录添加）
			pluginRegistry[i].Usage = image.HelpAddUsage() +
				"\n  添加表情 [关键词] + 图片   按关键词索引，用于空格触发" +
				"\n  " + quotationAdd + "   + 图片   按群+昵称索引，语录命令可查" +
				"\n  支持同时上传多张；引用含图片的消息也可触发"
			pluginRegistry[i].Commands = append(image.HelpAddCommands(), "添加表情", "添加语录")
		}
	}

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

func quotationHelpSyntax() (call, add string) {
	if config.C.Bot.CommandArgSpaceRequired {
		return "语录 <名字>", "添加语录 <昵称>"
	}
	return "语录[名字]", "添加语录[昵称]"
}

// ── Formatters ────────────────────────────────────────────────────────────────

var groupOrder = []string{"群管", "游戏", "提醒", "随机", "娱乐", "工具", "AI", "系统"}

// ── Register ───────────────────────────────────────────────────────────────────

func RegisterHelp(b *bot.Bot) {
	// 填充图片条目动态字段 + 构建索引（须在 image.Register 之后；见 main.go 注册顺序）。
	ensureRegistry()

	// Pre-render the list image in a background goroutine at startup so the
	// first user request is never blocked by font loading / rasterization.
	go func() {
		data, err := RenderHelpListImage()
		if err != nil {
			logx.Errorf("[help] image render failed: %v", err)
			return
		}
		helpListMu.Lock()
		helpListCache = data
		helpListMu.Unlock()
		logx.Infof("[help] image ready (%dKB)", len(data)/1024)
	}()

	b.OnCommand("help", "帮助").Plugin(catalog.PluginHelp).Handle(func(ctx *bot.CommandContext) error {
		if len(ctx.Args) == 0 {
			if !hfReady {
				return ctx.Reply("帮助图片不可用：请在 data/fonts/ 目录放入一个 TTF 或 OTF 字体文件后重启")
			}
			helpListMu.RLock()
			imageData := helpListCache
			helpListMu.RUnlock()
			if imageData == nil {
				return ctx.Reply("图片生成中，请稍后再试～")
			}
			return ctx.SendMsg(bot.Msg().ImageBytes(imageData).Build())
		}

		query := strings.Join(ctx.Args, " ")
		q := strings.TrimSpace(query)

		var entry *pluginEntry
		if id, err := strconv.Atoi(q); err == nil {
			entry = pluginByID[id]
		} else if p := pluginByName[strings.ToLower(q)]; p != nil {
			entry = p
		} else if p := pluginByCmd[strings.ToLower(q)]; p != nil {
			entry = p
		}

		if entry != nil {
			if !hfReady {
				return ctx.Reply("帮助图片不可用：请在 data/fonts/ 目录放入一个 TTF 或 OTF 字体文件后重启")
			}
			data, err := RenderHelpDetailImage(entry)
			if err != nil {
				return ctx.Reply("图片生成失败：" + err.Error())
			}
			return ctx.SendMsg(bot.Msg().ImageBytes(data).Build())
		}

		return ctx.Reply("未找到插件「" + query + "」，试试 帮助 查看完整清单")
	})
}
