# Cockpit — yueling-go

**Last updated**: 2026-06-06 by 月离 (landing：zssm 迁移 spec+plan 归档，无进行中任务)
**Active focus**: 无进行中开发任务；v1.1.0 已发布，zssm 迁移（spec+plan）已归档至 archive/

## 进行中

<!-- AUTO:inprogress -->
- [2026-06-06-external-message-api.md](specs/2026-06-06-external-message-api.md) — HTTP 服务凭 Bearer key 让外部系统发群/私聊消息，body 为 OneBot v11 段数组，同步返回 message_id；独立 services/httpapi 包，OnConnect 刷新活的 BotAPI
- [2026-06-06-external-message-api.md](plans/2026-06-06-external-message-api.md) — 实现 services/httpapi：Bearer 鉴权 POST /api/send，群/私聊，OneBot 段，atomic BotAPI 刷新；config [http_api]；SendPrivateMsg 返 message_id
<!-- /AUTO -->

## 下一步

（无进行中任务——等待下一个需求/想法启动。VL 模型配置背景见 incidents/2026-06-05-zssm-vl-image-support）

## Hanging tasks

- (none)
