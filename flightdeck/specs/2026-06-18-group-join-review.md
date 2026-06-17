---
status: active
summary: OnRequest(group) 按每群 db 配置的白名单(allow)/黑名单(deny)关键词审核加群申请：拒绝优先、命中通过、其余留人工；管理员命令 加群审核/加群白名单/加群黑名单 维护；退役全局 bot.join_keywords + 删 manager.go；顺带删搜ae、README 补 pack
last_updated: 2026-06-18
---

# 每群加群审核：白名单/黑名单关键词，退役全局 join_keywords

## 背景 / 目标

现有 `plugins/group/manager.go` 用全局 `bot.join_keywords` 对所有群统一 approve，且只能通过、不能拒绝、不能按群区分。目标：**每个群单独配置**加群审核规则，支持关键词命中**自动通过**与**自动拒绝**，运行时可改（不重启），管理员维护。参考 py 版 `yueling/plugins/group/manager/__init__.py`（每群 keyword→approve + `["*"]` 通配 + 硬编码拒绝），但要做完整：每群配置、白名单、黑名单。

成功标准：
- 某群配了白名单词，加群申请 comment 含该词 → 自动同意。
- 配了黑名单词，comment 含该词 → 自动拒绝。
- 白名单含 `*` → 任意非空 comment 即同意。
- 黑白名单都不命中 / 该群没配置 → 不处理，留管理员人工。
- 黑名单与白名单同时命中 → **拒绝优先**。
- 每群配置独立，互不影响；管理员命令增删查，改完即时生效。

## 数据模型（`db/db.go`）

```go
type GroupJoinRule struct {
    ID      uint  `gorm:"primaryKey"`
    GroupID int64 `gorm:"index"`
    Action  string // "allow" | "deny"
    Keyword string
}
```
- 进 `allModels`，建表走现有「`HasTable` 才 `CreateTable`」逻辑（零迁移风险）。
- CRUD：
  - `AddGroupJoinRule(groupID int64, action, keyword string) error`（重复则忽略/幂等）
  - `DeleteGroupJoinRule(groupID int64, action, keyword string) (bool, error)`
  - `GetAllGroupJoinRules() ([]GroupJoinRule, error)`

## 请求处理 + 决策（`plugins/group/join_review.go`，新）

内存 cache（照搬 `keyword.go`）：`map[int64]*joinRule{ allow, deny []string }`，注册时 `load()`，增删后 `reload()`，读写用 `sync.RWMutex`。

决策抽纯函数便于单测：
```go
type joinDecision int // decisionNone / decisionApprove / decisionReject
func decideJoin(comment string, allow, deny []string) joinDecision
```
规则（comment 已 `ToLower`）：
1. comment 为空 → `decisionNone`（无从匹配，留人工）。
2. 命中任一 `deny`（`strings.Contains`）→ `decisionReject`（**拒绝优先**）。
3. `allow` 含 `"*"`，或命中任一 `allow` 词 → `decisionApprove`。
4. 否则 `decisionNone`。

handler：
```go
b.OnRequest("group").Handle(func(ctx *bot.RequestContext) error {
    e := ctx.Event
    if e.SubType != "add" { return nil }
    rule := cache.get(e.GroupID)          // 无配置 → nil → 不处理
    if rule == nil { return nil }
    switch decideJoin(strings.ToLower(e.Comment), rule.allow, rule.deny) {
    case decisionReject:
        return ctx.SetGroupAddRequest(e.Flag, e.SubType, false, denyReason)
    case decisionApprove:
        return ctx.SetGroupAddRequest(e.Flag, e.SubType, true, "")
    }
    return nil
})
```
`const denyReason = "申请未通过审核"`（每群自定义理由 YAGNI，先不做）。

## 命令（均 `.Where(perm.Admin)`，群聊）

- `加群审核` — 列出本群 allow/deny 词 + 用法提示（无配置时提示如何添加）。
- `加群白名单 +<词>[,<词>...]` / `加群白名单 -<词>[,<词>...]` — 增/删通过词，**一条可加多个**（逗号分隔）；`+*` 设通配。无参数=列出白名单。
- `加群黑名单 +<词>[,<词>...]` / `加群黑名单 -<词>[,<词>...]` — 增/删拒绝词。无参数=列出黑名单。

每群每类的关键词数量不限（db 一行一个词，cache 为 `[]string`），反复添加累加。

解析：`strings.Join(ctx.Args, " ")` 后取首字符：`+`=增、`-`=删；其余部分按中英文逗号（`,` / `，`）分割成多个关键词，逐个 trim、去空、跳过重复。首字符非 +/- 或分割后无有效词 → 回用法提示。增删后 `reload()` cache 并回执（报实际增删了几个词）。

## 退役清理

- 删 `plugins/group/manager.go`；`cmd/bot/main.go` 把 `group.RegisterManager(b)` 换成 `group.RegisterJoinReview(b)`。
- `config/config.go`：移除 `BotConfig.JoinKeywords` 字段（全局已退役）。
- `plugins/system/help.go`：把「入群审批」(id 3) 改为「加群审核」+新命令（命令清单/Usage/Commands）。

## 顺带 chore（同一改动内）

- **删搜ae**：删 `plugins/tools/search_ae.go`；`main.go` 去掉 `tools.RegisterSearchAE(b)`；`help.go` 删 id 27「搜AE插件」；README 删 `搜ae插件/搜ae脚本` 两行；该插件那行 `ctx.React` 随文件删除。`checklists/2026-06-18-slow-command-progress-react` 里「搜ae」措辞顺手去掉。
- **README 补 pack**：工具区加 `pack`（图片打包）行；并新增「加群审核」节（命令 + 行为说明）。

## 边界 & 防滥用

- 只处理 `SubType=="add"`（用户主动申请）；`invite`（被邀请入群）不碰。
- comment 大小写不敏感（统一 ToLower 比较；关键词存储也 ToLower）。
- 没配置的群完全不介入，行为同现在「无人审批」。
- 高风险？拒绝/通过加群属可逆且非破坏，无需 `ConfirmRequired`；但命令限管理员（`perm.Admin`）。

## 不做（YAGNI）

- 每群自定义拒绝理由文案、按 UserID 黑白名单、正则匹配、申请人等级/入群验证问答。
- 全局兜底默认（已明确退役全局 join_keywords）。

## 测试

- `decideJoin` 纯函数：拒绝优先（同时命中 deny+allow）、通配 `*`、`contains` 命中、空 comment、无词配置 → 各自期望的 decision。
- db CRUD：Add（含幂等重复）/Delete（命中与未命中返回值）/GetAll 按 group 过滤。
- 命令解析：`+词`/`-词`/`+词1,词2`（逗号多词，含中文逗号）/无参/首字符非 +/-/空词 各分支。
- 端到端审批依赖 NapCat，手动验证（部署后造一条加群申请测通过/拒绝）。
