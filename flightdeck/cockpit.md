# Cockpit — yueling-go

**Last updated**: 2026-06-18 by 月离 (新增可配置 AI 频率限制：每人/每群每分钟上限 [ai.ratelimit]，默认 5/15，0=不限，作用于群聊AI/zssm/翻译，后台调用不计入；另把加群审核命令改为覆盖语义)
**Active focus**: 本次新增可配置 AI 调用频率限制（每人+每群，双窗口）+ 加群审核命令改覆盖语义；两项均待重新部署后手验；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->

<!-- /AUTO -->

## 下一步

无进行中开发任务。待重新部署后手验两项：

1. **AI 频率限制** — 配 `[ai.ratelimit] user_per_min=5, group_per_min=15`，同一人 1 分钟内连发 >5 次群聊AI/zssm/翻译 → 回「你发消息太频繁了…」；群内多人累计 >15 次/分钟 → 回「本群 AI 用得太频繁了…」；改 0 则不限。
2. **加群审核新命令（覆盖语义）** — `加群审核` 展示白名单/黑名单；`加群白名单 交流,学习` 直接覆盖；`加群白名单`（空）清空；命中通过词自动通过、命中拒绝词自动拒绝、其余留人工。

其余五项（pack 上传/嵌套转发/进度表情/设精/复读不复读命令）此前已手验通过。

## Pending Review

- ⚠待手验: AI 频率限制 — aiLimiter 双窗口逻辑已过单测（ai/ratelimit_test.go：个人/群超限、0=不限、被拦不占名额、私聊跳群窗），但真实群里触发提示 + config 默认 5/15 是否符合预期未在线上验证；重新部署后手验「下一步 1」。
- ⚠待手验: 加群审核（覆盖语义）— joinListHandler 改为 db.SetGroupJoinRules 一次性覆盖、parseKeywords 去掉 +/-，已过单测（TestParseKeywords + db TestGroupJoinRuleCRUD），但整条 OnRequest 审批链路 + 新命令交互未在线上验证；重新部署后手验「下一步 2」确认查看/覆盖/清空/自动通过拒绝。

## Hanging tasks

- (none)
