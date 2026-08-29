# 群聊统计与聊天洞察

## Goal

在远程最新基线上完成并验证群聊统计整条功能，包括 Bot 命令、AI 工具、PostgreSQL 查询、WebUI API 与 Vue 聊天洞察页面；保持按群隔离、无需 AI，并在真实数据量下维持可接受的查询延迟。

## Status

Open

## Current

本地 `main` 已快进到 `origin/main` 的 `v1.18.6`，未提交的 Go/Vue 群聊洞察功能已恢复并解决上游冲突。Bot 命令、AI 工具、页面、API、长期记录和按群清理已完成；活跃用户的原句/词频已从最多约 19 条 SQL 改成固定 5 条。批量词频与重复原句已经在真实 `zhparser` PostgreSQL 上通过，30,000 条消息、8 位用户的五查询基线约为 295 ms/op；词云异步挂载后的 resize 监听和过期布局回写也已修正。尚缺连接 NapCat/QQ群后的完整手验和生产数据量延迟。

## Next

连接 NapCat 后手验群切换、四种时间范围、词云、群友原句回退和按群历史清理；若真实 30 天接口明显慢于 [`BenchmarkPostgresChatInsightQueries`](../../../db/chatstats_benchmark_test.go) 的本地基线，再按[聊天洞察实践](../../knowledge/chat/chat-insights.md)采集 `EXPLAIN (ANALYZE, BUFFERS)`，有证据后才调整索引或缓存。

## Progress

- 将本地开发基线从 `v1.16.0` 快进到远程 `v1.18.6`，保留全部聊天洞察改动。
- 合并时采用上游更新的 Go 词云像素碰撞布局和群选择框样式。
- 新增 `/chat-insights` 页面、群/时间范围选择、词云、活跃榜和群友常说的话。
- 新增按群历史统计与显式清理，取消旧的 35 天全局自动删除。
- 统一 bot 命令、AI 工具和 WebUI 的停用词与冗余词筛选。
- 使用两条按 `user_id` 分区的批量窗口查询消除 WebUI N+1。
- 在真实 `zhparser` PostgreSQL 上覆盖批量词频与重复原句查询，并加入 30,000 条消息的五查询 benchmark。
- 修复 Vue 词云异步出现后未监听尺寸变化，以及旧布局结果可能写回空页面的问题。
- 恢复 Dashboard 的日报快捷入口，将 D3 词云和历史清理分别提取为独立模块；页面、后台外壳及共享 Vue 组件的专用样式已迁为所属文件内的 Tailwind utilities，`main.css` 从 2228 行收敛到 330 行，只保留 token、基础 reset、跨页面共享模块和 Nuxt UI 全局覆盖，删除期间的弹窗与范围控件会统一锁定。
- 完成相关 Go 测试、静态检查和前端生产构建；全量测试仅剩本机缺中文字体导致的两个上游图片测试失败。
- 安装 Flightdeck `3.0.0-alpha.8` 并把当前恢复入口迁到新版模型。

## References

- [设计记录](design.md) — WebUI 总体架构及聊天洞察扩展的设计背景。
- [README 聊天统计说明](../../../README.md) — 面向用户的命令和 WebUI 行为。
- [WebUI Tailwind-first 实践](../../knowledge/webui/tailwind-first.md) — 新页面的样式放置和例外边界。
