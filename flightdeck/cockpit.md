# Cockpit — yueling-go

**Last updated**: 2026-06-15 by 月离 (超管满足 Admin/Owner 权限条件，已合并入 main)
**Active focus**: 超管免群权限门槛已完成并合入 main（cond.Admin/Owner 放行 config superusers）；pack 功能此前亦已合入；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->
- [2026-06-15-set-essence.md](specs/2026-06-15-set-essence.md) — 设精命令——回复一条消息发「设精/加精」把它加入群精华，普通用户即可用（不加 perm.Admin）；新增 bot.SetEssenceMsg 封装 set_essence_msg + plugins/group/essence.go
- [2026-06-15-set-essence.md](plans/2026-06-15-set-essence.md) — 设精命令实现计划——bot.SetEssenceMsg 封装 + plugins/group/essence.go + 注册，2 任务（build+手验，无单测面）
<!-- /AUTO -->

## 下一步

无进行中任务。main 累积多个未 push 提交（pack 功能 + 超管权限 + 着陆，push 按惯例待确认）。超管权限改动纯逻辑、已单测覆盖，无需额外手验。pack 端到端待部署后手验：回复带图消息 / 合并转发发 `pack` → 群文件出 zip。

## Hanging tasks

- (none)
