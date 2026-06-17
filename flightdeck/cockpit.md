# Cockpit — yueling-go

**Last updated**: 2026-06-18 by 月离 (新增耗时命令进度表情：SetMsgEmojiLike + ctx.React(bot.EmojiProcessing="424")，接入 pack/zssm/翻译/场景识别/搜ae)
**Active focus**: v1.4.0 已发布；pack 两处 bug 已修；新增耗时命令进度表情提示；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->

<!-- /AUTO -->

## 下一步

无进行中任务。三项改动待重新部署后手验：① pack 上传 — 回复带图/合并转发发 `pack` → 群文件出 zip（之前报「识别URL失败」，base64:// 已修）；② pack 嵌套转发 — 回复「合并转发里又套合并转发」发 `pack` → 内层图也进 zip（之前只取第一层，见 incidents/2026-06-18-pack-nested-forward-inline-content）；③ 耗时命令进度表情 — 发 `pack`/`zssm`/`翻译`/`场景识别`/`搜ae` → 命令消息上立刻出现「处理中」表情（424，见 checklists/2026-06-18-slow-command-progress-react）；④ 设精 — 回复消息发 `设精`/`加精` → 进群精华（需 bot 是群管理员）。

## Pending Review

- ⚠待手验: incidents/2026-06-18-pack-nested-forward-inline-content — 单测已守护（TestCollectImagesInlineForward），但嵌套转发的真实 NapCat 形状未在线上验证；部署后跑手验清单②确认。
- ⚠待手验: 耗时命令进度表情 — emoji_id="424" 与 set_msg_emoji_like 在线上 NapCat 的实际表现未验证；部署后跑手验清单③确认表情正常显示。

## Hanging tasks

- (none)
