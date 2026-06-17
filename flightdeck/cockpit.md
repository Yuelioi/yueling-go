# Cockpit — yueling-go

**Last updated**: 2026-06-18 by 月离 (修 pack 嵌套合并转发漏图：内层 forward 无可二次查询 id，子消息内联 data.content，collectImages 改优先吃 content)
**Active focus**: v1.4.0 已发布；pack 两处 bug 已修（base64:// 上传 + 嵌套转发取图）；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->

<!-- /AUTO -->

## 下一步

无进行中任务。pack 两处 bug 已修（base64:// 上传、嵌套合并转发取图），待重新部署后手验。部署后手验清单：① pack 上传 — 回复带图/合并转发发 `pack` → 群文件出 zip（之前报「识别URL失败」，base64:// 已修）；② pack 嵌套转发 — 回复「合并转发里又套合并转发」发 `pack` → 内层图也进 zip（之前只取第一层，见 incidents/2026-06-18-pack-nested-forward-inline-content）；③ 设精 — 回复消息发 `设精`/`加精` → 进群精华（需 bot 是群管理员）。

## Pending Review

- ⚠待手验: incidents/2026-06-18-pack-nested-forward-inline-content — 单测已守护（TestCollectImagesInlineForward），但嵌套转发的真实 NapCat 形状未在线上验证；部署后跑手验清单②确认。

## Hanging tasks

- (none)
