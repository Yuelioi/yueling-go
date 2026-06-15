# Cockpit — yueling-go

**Last updated**: 2026-06-15 by 月离 (pack 命令：回复消息发 pack→递归抽合并转发图片打 zip 上传群文件，已合并入 main)
**Active focus**: pack 功能已完成并合入 main（bot.GetForwardMsg + plugins/tools/pack.go + [pack] 可配上限 100/100）；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->
- [2026-06-15-superuser-admin-bypass.md](specs/2026-06-15-superuser-admin-bypass.md) — 让 cond.Admin / cond.Owner 条件对 config.C.Bot.SuperUsers 也放行，超管无需群管/群主身份即可用禁言/撤回等管理命令；一处改动覆盖所有 .Where(perm.Admin) 命令
- [2026-06-15-superuser-admin-bypass.md](plans/2026-06-15-superuser-admin-bypass.md) — 超级管理员免群权限门槛实现计划——改 cond.Admin/Owner 放行 SuperUsers，单任务 TDD
<!-- /AUTO -->

## 下一步

无进行中任务。pack 已合入 main 但**尚未 push**（push 按惯例待确认）。pack 端到端依赖 NapCat，待部署后手验：回复带图消息 / 合并转发发 `pack` → 群文件出 zip；上限按需在 `config.toml` 加 `[pack] max_images/max_mb` 调整。

## Hanging tasks

- (none)
