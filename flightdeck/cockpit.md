# Cockpit — yueling-go

**Last updated**: 2026-06-18 by 月离 (发布 v1.5.0：pack 流式上传+嵌套转发修复、连接层防 panic、耗时命令进度表情、help 补登记；README 加致谢)
**Active focus**: v1.5.0 已发布（已 push + tag）；待线上手验三项（见 Pending Review）；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->
- [2026-06-18-group-join-review.md](specs/2026-06-18-group-join-review.md) — OnRequest(group) 按每群 db 配置的白名单(allow)/黑名单(deny)关键词审核加群申请：拒绝优先、命中通过、其余留人工；管理员命令 加群审核/加群白名单/加群黑名单 维护；退役全局 bot.join_keywords + 删 manager.go；顺带删搜ae、README 补 pack
<!-- /AUTO -->

## 下一步

无进行中任务。待重新部署后手验：① pack 大文件上传 — 回复图多的消息发 `pack` → 群文件出 zip 且**连接不再断**（之前 base64:// 大包触发 close 1009 断连+panic，现走 upload_file_stream 流式分片，见 incidents/2026-06-18-pack-upload-stream-1009）；② pack 嵌套转发 — 回复「合并转发里又套合并转发」发 `pack` → 内层图也进 zip（见 incidents/2026-06-18-pack-nested-forward-inline-content）；③ 耗时命令进度表情 — 发 `pack`/`zssm`/`翻译`/`场景识别`/`搜ae` → 命令消息上立刻出现「处理中」表情（424，见 checklists/2026-06-18-slow-command-progress-react）；④ 设精 — 回复消息发 `设精`/`加精` → 进群精华（需 bot 是群管理员）。

## Pending Review

- ⚠待手验: incidents/2026-06-18-pack-nested-forward-inline-content — 单测已守护（TestCollectImagesInlineForward），但嵌套转发的真实 NapCat 形状未在线上验证；部署后跑手验清单②确认。
- ⚠待手验: 耗时命令进度表情 — emoji_id="424" 与 set_msg_emoji_like 在线上 NapCat 的实际表现未验证；部署后跑手验清单③确认表情正常显示。
- ⚠待手验: incidents/2026-06-18-pack-upload-stream-1009 — upload_file_stream 整条链路（分片→合并→file_path→upload_group_file）仅单测覆盖，未在线上 NapCat 实测；部署后跑手验清单①确认大包能上传且连接不断。需 NapCat ≥ v4.8.115（compose 用 :latest，满足）。

## Hanging tasks

- (none)
