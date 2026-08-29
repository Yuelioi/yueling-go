# 群聊洞察与聊天历史

## 边界与数据来源

- `plugins/funny/chatstats.go` 以高优先级监听群消息，在其他命令 handler 之前写入 `group_chat_messages`；写入失败只记录 `logx.Warnf`，不阻断消息分发。
- 实时事件和 NapCat 历史补取可能重叠，唯一键 `(group_id, message_id)` 配合 `ON CONFLICT DO NOTHING` 保证幂等。
- 只提取文字，内容裁到 2,000 个 rune，昵称裁到 32 个 rune。词云、榜单、口头禅等统计命令自身仍会保存，但标成 `stat_excluded`，不会污染统计。
- 记录长期保存在本地 PostgreSQL，不再自动执行旧的 35 天全局清理。WebUI 只允许按指定群预览并删除某个时间点以前的记录，或清空该群；不能跨群删除。
- 统计完全本地执行，不把聊天正文发给 AI。所有读取必须同时限定 `group_id` 和时间范围；个人统计再追加 `user_id`。

## PostgreSQL 结构与查询

`group_chat_messages` 的关键索引：

- `(group_id, created_at)`：群级时间范围统计。
- `(group_id, user_id, created_at)`：个人时间范围统计。
- `search_vector` GIN：由 `public.chinese_zhparser` 生成中文全文向量。
- `content gin_trgm_ops`：`谁最爱说` 的字面子串检索。

词云使用 `tsvector_to_array(search_vector)` 展开每条消息的 lexeme，再按词计数。`SelectGroupChatWords` 在 Go 中做停用词、最低频次、长词权重和包含关系去重；不要在 Go 中重复实现中文分词。

“常说原句”按 `TRIM(content)` 精确分组，仅保留 2–48 字且至少重复两次的内容；没有稳定原句时，WebUI 回退显示个人高频词。

## WebUI 契约

- `GET /api/webui/chat-insights?group_id=<id>&period=today|yesterday|7days|30days`
  返回摘要、群词云、最多 8 位活跃成员，以及每人的重复原句/高频词。
- `GET /api/webui/groups/:groupID/chat-history?before_at=<unix>`
  返回该群总数、预计删除数和最早/最新时间。
- `DELETE /api/webui/groups/:groupID/chat-history`
  请求体只能二选一：`{"before_at": <unix>}` 或 `{"all": true}`。

页面入口是 `/chat-insights`。词云布局由浏览器端 `d3-cloud` 完成；接口最多返回 36 个群级词，布局计算不是服务端性能瓶颈。

## 性能约束

不要在活跃用户循环里逐人查询原句和高频词。8 位用户会把一个页面请求放大到最多约 19 条 SQL。

当前实现先取用户 ID，再调用：

- `GetGroupChatTopPhrasesForUsers`：一次窗口函数查询，按 `user_id` 分区取每人前 N 条原句。
- `GetGroupChatTopWordsForUsers`：一次窗口函数查询，按 `user_id` 分区取每人前 N 个 lexeme。

因此非空页面固定约 5 条 SQL：摘要、群词、活跃用户、批量原句、批量用户词。新增字段时优先继续批量聚合，不要恢复 N+1 查询。

## 验证

- 代码级：`go test ./db ./services/webui ./ai/tools ./plugins/funny ./plugins/system`
- 静态检查：`go vet ./db ./services/webui ./plugins/funny`
- 前端：`pnpm --dir webui run build`
- PostgreSQL 实测需要设置 `YUELING_TEST_DATABASE_DSN`，目标库必须支持 `zhparser` 与 `pg_trgm`。`TestPostgresZhparserChatQueries` 同时覆盖批量用户词和批量重复原句查询。
- 性能基线使用 `go test ./db -run '^$' -bench '^BenchmarkPostgresChatInsightQueries$' -benchtime=3x`；它生成 30,000 条、8 位用户的隔离数据并执行页面所需的固定 5 条 SQL。不要把不同机器的绝对值当硬门槛，只比较同一环境中的查询计划和相对变化。
