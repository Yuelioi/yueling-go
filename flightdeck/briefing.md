# briefing — yueling-go

## Conventions

- 日志统一用 `services/logx`，不用 stdlib `log`（见 `knowledge/logging/logx.md`）。
- commit / push：本地 commit agent 自调（可 reset/amend）；**push 先问我**。
- 任务彻底完成后，自动执行 persist/landing（更新 cockpit / 分类知识 / 本地 commit），
  无需我手动触发；push 仍按默认先问。
  <!-- flightdeck 无原生 auto-land 开关，此为本 deck 约定：会话内一个任务彻底收尾时主动 persist。 -->

## Subscriptions

knowledge/commits.md
knowledge/comments.md
<!-- commits/comments playbook 都是项目无关的通用规范，订阅母库 ~/.flightdeck/knowledge/。
     commits.md 含 §6 多行 message/here-string、§7 暂存 RM/MM。 -->
