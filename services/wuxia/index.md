# 武侠游戏模块

此模块拥有按群角色、随机历练、职业武器、战斗和独立银两的规则与持久化。调用方通过 `New` 构造后使用 `Execute`、`Profile`、`Ranking`、`LatestResult` 和配置／流水查询，不直接改数据库角色余额。

- `model.go` — 角色、职业、武器、事件、对局快照和回执的数据表示。
- `internal/content/` — 内容定义、职业/等级资格校验与固定历史定义。
- `internal/progression/` — 等级上限与职业基础/逐级成长计算。
- `preview.go`、`internal/content/equipment_preview.json` — 独立待发布装备内容及无持久化试配。
- `weapon_preview_draw.go` — 分品质后分定义的武器试抽，独立于正式银两奖池。
- `balance_report.go`、`equipment_balance.json`、`equipment_preview_test.go` — 后台平衡报告与可复现组合/战斗模拟。
- `catalog.go` — 角色属性与展示快照的装配。
- `internal/combat/`、`battle.go` — 共享真实气血结算的历练与切磋；种子、属性与规则版本组成回放依据。
- [战斗规则](combat.md) — 武技、心法、兵器联动、事件与寻宝接入契约。
- [六职业候选](six_professions.md) — 职业兵器、武技、成长与离线验证入口；正式接入前的内容边界。
- `six_professions_test.go`、`six_professions_balance.json` — 六职业完整配对、成长、奖池覆盖与独立种子验证。
- `martial_test.go`、`martial_balance.json` — 气血守恒、施展窗口、品质价值与分等级跨职业审计，以及学习槽位和回执验证。
- `inventory.go` — 发奖、穿戴、学习与自动换装的编排。
- `internal/assets/`、`internal/gameplay/` — 原子扣款/入库与纯加权抽奖。
- `commands.go`、`actions.go` — 类型化命令与独立玩法处理器。
- `casino.go`、`casino_test.go` — 三骰组合倍率规则、银两下注及重复消息结算验证。
- `receipt.go` — 回执读取与旧寻宝快照解码。
- `upgrade_test.go`、`architecture_test.go` — 等级/职业筛选、旧角色迁移、快照和模块依赖约束。
- `inventory_test.go` — 旧角色库存迁移、装备武学效果、扣款与重复消息回归。
- [内容扩展](extending.md) — 新装备、武学和奖池的配置方法与新增效果边界。
- `adventure.go` — 加权事件选择、敌人、奖励与武器收集。
- `service.go` — 操作事务、消息回执、每日记录、银两账本和全局／群配置继承。
- `guidance.go` — 根据签到、历练、银两和武学状态推荐可执行的下一步。
- [小群试玩](playtest.md) — 开服配置、玩家入门、完整试玩路线和本地验证入口。
- `*_test.go` — 平衡模拟、扩展内容、PostgreSQL并发与事务行为。
- [本地角色卡](cards/index.md) — 纯服务器排版和静态美术资源。

新职业、武器和既有事件类型的内容优先增加到 `internal/content/content.json`，旧ID不重新分配。新效果需要在战斗规则模块中显式实现并提升规则版本。回执始终返回已经保存的结果，不随新版本重掷随机。数据库结构由 `db/migrations` 管理，不运行自动建表。

- `adventure_bonus_test.go` — 饰品历练收益边界与候选气血、武器品质预算验证。

- `acquisition_test.go` — 重复收藏折银、溢出原子性与高品质自动换装规则验证。
