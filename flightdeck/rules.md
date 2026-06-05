---
version: 3.0
---

## House rules
<!-- 默认：本地 commit 自调(可 reset/amend) + push 先问 + landing 智能归档；脚本/ git / AGENTS 均自动推断。
     极少需要改；要改就在下方 heading 下打一行短语。可用短语：
       commit: ask | don't auto-commit
       status: don't auto start
       this deck doesn't use git | has AGENTS.md but don't auto-regen -->

### Project conventions

- 日志统一用 `services/logx`，不用 stdlib `log`（见 checklists/2026-06-05-logging）。

### Autonomy overrides

- 任务彻底完成后，自动执行 landing 仪式（更新 cockpit / 分类知识 / 本地 commit），无需我手动 `/flightdeck:landing`；push 仍按默认先问我。
  <!-- 注：flightdeck 3.0 无原生 auto-land 开关，此为 agent 约定：会话内一个任务彻底收尾时主动跑 landing。 -->
