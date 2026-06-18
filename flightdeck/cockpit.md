# Cockpit — yueling-go

**Last updated**: 2026-06-18 by 月离 (六项手验全部通过，看板清空；本次将加群审核命令简化为覆盖语义：加群白名单/黑名单 词1,词2 直接覆盖、留空清空，去掉 +/- 增删，db 新增事务性 SetGroupJoinRules)
**Active focus**: 六项手验全部通过；本次把加群审核命令改为覆盖语义（不再 +/- 增删），待重新部署后手验该命令；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->

<!-- /AUTO -->

## 下一步

无进行中开发任务。待重新部署后手验 **加群审核新命令（覆盖语义）**：`加群审核` → 展示本群白名单 / 黑名单；`加群白名单 交流,学习` → 直接覆盖通过词；`加群白名单`（空）→ 清空；`加群黑名单 广告` 同理；命中通过词的入群理由自动通过、命中拒绝词自动拒绝、其余留人工。其余五项（pack 上传/嵌套转发/进度表情/设精/复读不复读命令）此前已手验通过。

## Pending Review

- ⚠待手验: 加群审核（覆盖语义）— joinListHandler 改为 db.SetGroupJoinRules 一次性覆盖、parseKeywords 去掉 +/-，已过单测（TestParseKeywords + db TestGroupJoinRuleCRUD），但整条 OnRequest 审批链路 + 新命令交互未在线上验证；重新部署后手验「下一步」确认查看/覆盖/清空/自动通过拒绝。

## Hanging tasks

- (none)
