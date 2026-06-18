# Cockpit — yueling-go

**Last updated**: 2026-06-18 by 月离 (每群加群审核落地：db GroupJoinRule + 加群审核/白名单/黑名单命令 + OnRequest 决策，退役全局 join_keywords；删搜ae；README 补 pack)
**Active focus**: v1.6.0 已发布（push + tag）：每群加群审核 + 删搜ae；待线上手验四项（见 Pending Review）；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->

<!-- /AUTO -->

## 下一步

无进行中任务。待重新部署后手验：① pack 大文件上传 — 回复图多的消息发 `pack` → 群文件出 zip 且**连接不再断**（之前 base64:// 大包触发 close 1009 断连+panic，现走 upload_file_stream 流式分片，见 incidents/2026-06-18-pack-upload-stream-1009）；② pack 嵌套转发 — 回复「合并转发里又套合并转发」发 `pack` → 内层图也进 zip（见 incidents/2026-06-18-pack-nested-forward-inline-content）；③ 耗时命令进度表情 — 发 `pack`/`zssm`/`翻译`/`场景识别` → 命令消息上立刻出现「处理中」表情（424，见 checklists/2026-06-18-slow-command-progress-react）；④ 设精 — 回复消息发 `设精`/`加精` → 进群精华（需 bot 是群管理员）；⑤ 加群审核 — 某群 `加群白名单 +交流`，再用含「交流」理由申请入群→自动通过；`加群黑名单 +广告`，含「广告」理由→自动拒绝；`加群审核` 查看本群配置（见 archive/specs/2026-06-18-group-join-review）。

## Pending Review

- ⚠待手验: incidents/2026-06-18-pack-nested-forward-inline-content — 单测已守护（TestCollectImagesInlineForward），但嵌套转发的真实 NapCat 形状未在线上验证；部署后跑手验清单②确认。
- ⚠待手验: 耗时命令进度表情 — emoji_id="424" 与 set_msg_emoji_like 在线上 NapCat 的实际表现未验证；部署后跑手验清单③确认表情正常显示。
- ⚠待手验: incidents/2026-06-18-pack-upload-stream-1009 — upload_file_stream 整条链路（分片→合并→file_path→upload_group_file）仅单测覆盖，未在线上 NapCat 实测；部署后跑手验清单①确认大包能上传且连接不断。需 NapCat ≥ v4.8.115（compose 用 :latest，满足）。
- ⚠待手验: 每群加群审核 — decideJoin/parseKeywordArg/db CRUD 已单测，但 OnRequest 加群审批链路依赖真实 NapCat 事件，未实测；部署后跑手验清单⑤确认通过/拒绝/命令回执。

## Hanging tasks

- (none)
