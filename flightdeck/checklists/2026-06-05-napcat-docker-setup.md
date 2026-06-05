---
status: active
when_to_read: 首次用 docker-compose 起 NapCat、或排查 bot 连不上协议端时
applies_to: [napcat, docker, onebot]
last_updated: 2026-06-05
---

# NapCat docker 首次配置

`docker-compose.yml` 已含 `napcat` 服务（`mlikiowa/napcat-docker`），与 `bot` 同处 compose 默认网络，可用服务名互通。QQ 登录无法脚本化，首次需手动：

1. `docker compose up -d`
2. 浏览器打开 `http://<宿主机IP>:6099/webui`，登录 token 见 `docker logs yueling-napcat`（默认 `napcat`）
3. 扫码登录 QQ（登录态持久化在 volume `napcat-qq` → `/app/.config/QQ`，重启免重扫）
4. WebUI → 网络配置 → 新建「WebSocket 客户端」→ 地址 `ws://bot:9077/onebot/v11/ws`，token 与 `config.toml` 的 `[napcat].token` 一致 → 启用
5. `config.toml` 设 `[napcat] serve = ":9077"`（反向 WS，推荐）

## 关键点（核对过上游 Dockerfile/entrypoint）

- 声明的持久化 VOLUME 只有 `/app/.config/QQ` 与 `/app/napcat/config`；plugins 路径**不是** VOLUME。
- `NAPCAT_UID/GID` 上游 entrypoint 默认 **0(root)**（`: ${NAPCAT_UID:=0}`），Linux 想用宿主机权限再覆盖为 `$(id -u)`。
- 反向 WS 下 NapCat 是出站客户端，**不需要**任何入站 OneBot 端口；只暴露 `6099` WebUI 即可。
- **不要**给 napcat 设 `network_mode: bridge`（上游文档示例那样会断掉 compose 服务名 DNS，bot 就连不上 `napcat`/NapCat 连不上 `bot`）。
- 正向 WS 替代：WebUI 启「WebSocket 服务器」，`config.toml` 设 `url = "ws://napcat:3001"`。

相关：[[2026-06-05-zssm-vl-image-support]]（同属 bot 运行时配置）。
