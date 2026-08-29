# 群聊统计与聊天洞察上下文

## 范围

WebUI 由 Go/Gin 与 bot 同进程提供服务，Vue/Vite 只负责构建浏览器静态资源。运行期管理数据和聊天记录存放在 PostgreSQL，不写回通常以只读方式挂载的 `config.toml`。

当前 Work 只收尾聊天洞察及其真实环境验证。既有插件策略、命令统计、消息发送、AI 风格、日报、订阅、知识库、好感度和长期记忆页面继续保持现状。

## 聊天数据边界

- `group_chat_messages` 以 `(group_id, message_id)` 幂等写入实时消息与 NapCat 历史补取。
- 每次统计读取必须限定 `group_id` 和时间范围；个人分析再限定 `user_id`。
- 统计命令自身标记为 `stat_excluded`，不进入词云、榜单和口头禅。
- 原始记录默认长期保留，只能通过 WebUI 对当前选中群预览并显式清理。
- 词云和口头禅使用 PostgreSQL `zhparser` 与本地 Go 筛选，不把聊天正文发给 AI。

## 性能决策

群级摘要、群词频和活跃用户分别查询一次。最多八位活跃用户的重复原句与高频词各使用一次 `ROW_NUMBER() OVER (PARTITION BY user_id ...)` 批量查询，禁止在用户循环内恢复逐人 SQL。

前端 `d3-cloud` 只摆放服务端返回的最多 36 个词，不承担分词、计数或访问控制。

## 验证环境

PostgreSQL 集成测试通过 `YUELING_TEST_DATABASE_DSN` 创建隔离 schema，并要求数据库可创建/使用 `zhparser` 与 `pg_trgm`。本机可用临时 `yueling-postgres:16-zhparser` 容器运行测试，但不保留常驻测试数据库。

全量 Go 测试中的 `plugins/avatar_meme/internal/templates/single_plan` 两个失败来自本机缺少中文字体，与聊天洞察无关；相关包的定向测试已经通过。

## 完成条件

- 批量词频与原句 SQL 在真实 PostgreSQL 上通过集成测试。
- 30 天真实数据下记录冷/热接口延迟，确认没有明显回归，或基于执行计划完成有证据的修复。
- 连接 NapCat 后手验群切换、四种时间范围、词云、原句回退和按群历史清理。
