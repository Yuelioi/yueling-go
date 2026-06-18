# Cockpit — yueling-go

**Last updated**: 2026-06-18 by 月离 (手验① 确认 1009/panic 根治，修 RC-C 上传超时误报(callT 180s)；另修复读插件复读命令：dispatcher 对 Command/FullMatch 命中置 commandMatched，复读跳过，零维护取代黑名单，CLAUDE.md 加注册约定)
**Active focus**: v1.6.1 已发布（push + tag）：修 RC-C 上传超时误报 + 复读不复读命令；待重新部署后手验六项（见 Pending Review，①RC-C 复测 ②③④⑤ ⑥复读命令）；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->

<!-- /AUTO -->

## 下一步

无进行中任务。待重新部署后手验：① pack 大文件上传 — 回复图多的消息发 `pack` → 群文件出 zip：**连接不再断已确认✅**（1009/panic 已根治），本次需复测 **不再误报「上传失败」**（RC-C：之前 10s call 超时在 ~20s 上传仍在进行时误报，现上传走 callT 180s，见 incidents/2026-06-18-pack-upload-stream-1009 Case 2）；② pack 嵌套转发 — 回复「合并转发里又套合并转发」发 `pack` → 内层图也进 zip（见 incidents/2026-06-18-pack-nested-forward-inline-content）；③ 耗时命令进度表情 — 发 `pack`/`zssm`/`翻译`/`场景识别` → 命令消息上立刻出现「处理中」表情（424，见 checklists/2026-06-18-slow-command-progress-react）；④ 设精 — 回复消息发 `设精`/`加精` → 进群精华（需 bot 是群管理员）；⑤ 加群审核 — 某群 `加群白名单 +交流`，再用含「交流」理由申请入群→自动通过；`加群黑名单 +广告`，含「广告」理由→自动拒绝；`加群审核` 查看本群配置（见 archive/specs/2026-06-18-group-join-review）；⑥ 复读不复读命令 — 在群里连发 3 次 `pack`（或 `我老婆呢` 等任意命令）→ bot **不**复读该命令（普通聊天如「哈哈」连发 3 次仍正常复读）。

## Pending Review

- ⚠待手验: incidents/2026-06-18-pack-nested-forward-inline-content — 单测已守护（TestCollectImagesInlineForward），但嵌套转发的真实 NapCat 形状未在线上验证；部署后跑手验清单②确认。
- ⚠待手验: 耗时命令进度表情 — emoji_id="424" 与 set_msg_emoji_like 在线上 NapCat 的实际表现未验证；部署后跑手验清单③确认表情正常显示。
- ⚠待复测: incidents/2026-06-18-pack-upload-stream-1009 — 手验① 已确认 1009/panic 根治、连接不断✅；但暴露 RC-C（10s call 超时误报「上传失败」），已修（上传走 callT 180s）+ 守护测试 TestCallTHonorsTimeout，需重新部署后复测①确认大包不再误报。需 NapCat ≥ v4.8.115（compose 用 :latest，满足）。
- ⚠待手验: 每群加群审核 — decideJoin/parseKeywordArg/db CRUD 已单测，但 OnRequest 加群审批链路依赖真实 NapCat 事件，未实测；部署后跑手验清单⑤确认通过/拒绝/命令回执。
- ⚠待手验: 复读不复读命令 — dispatcher commandMatched 分类已单测（TestIsCommandMatcher），但整条复读链路未在线上验证；部署后跑手验清单⑥确认命令不复读、普通聊天仍复读。

## Hanging tasks

- (none)
