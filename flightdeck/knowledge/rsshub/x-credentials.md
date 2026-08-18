---
kind: note
summary: "RSSHub X 路由的两种认证方式、凭证来源，以及 docker-compose 配置核对。"
activation: reference
read_when: "配置或排查 RSSHub 的 X/Twitter 用户订阅时。"
---

# RSSHub X 路由凭证

核对日期：2026-08-18。RSSHub 上游基线：`697421be62613f3d1db960f53adb8cd569343a9c`。

## 结论

`TWITTER_AUTH_TOKEN` 与 `TWITTER_CONSUMER_KEY` / `TWITTER_CONSUMER_SECRET` **不是需要同时填写的一组三件套**，而是两种替代认证方式：

| 方式 | RSSHub 配置 | 是什么 | 从哪里取得 |
| --- | --- | --- | --- |
| X Web API（RSSHub 推荐） | 仅 `TWITTER_AUTH_TOKEN` | 已登录 X 网页会话的 `auth_token` Cookie，不是 X Developer API 的 Bearer Token | 登录 `x.com` 后，从浏览器开发者工具的 Cookies 中读取 `auth_token` 的值；不需要申请开发者 App |
| X Developer API | `TWITTER_CONSUMER_KEY` + `TWITTER_CONSUMER_SECRET` | X 现在称为 **API Key and Secret**，用于标识开发者 App；不是 Access Token，也不是 Bearer Token | 在 [X Developer Console](https://console.x.com/) 注册开发者账号、创建 App，然后在 App 的 **Keys and tokens** 页面生成/查看 |

RSSHub 的官方路由说明明确把 Web Cookie 与 Developer API 列为两个认证方法，并称 Developer API 路径为 Pay-Per-Use：[RSSHub `namespace.ts` 第 49–53 行](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/twitter/namespace.ts#L49-L53)。X 官方文档说明 API Key/Secret 也叫 Consumer Key/Secret，并给出开发者账号、App、Keys and tokens 的取得流程：[API Key and Secret](https://docs.x.com/fundamentals/authentication/oauth-1-0a/api-key-and-secret)、[Getting Access](https://docs.x.com/x-api/getting-started/getting-access)。

Web API 实现会把配置值直接组装为 `auth_token=<值>` Cookie，也印证它不是开发者 API 凭证：[RSSHub `web-api/utils.ts` 第 15–22 行](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/twitter/api/web-api/utils.ts#L15-L22)。

`TWITTER_AUTH_TOKEN` 等同于完整浏览器登录会话的敏感凭证，应像密码一样保护，不要写入仓库、日志或聊天记录。RSSHub 支持用逗号分隔多个 `auth_token` Cookie；本项目只需要一个时直接填单个值即可。

## 是否必须同时配置三个值

不需要。当前 RSSHub 运行时代码的选择逻辑是：

1. 有 `TWITTER_AUTH_TOKEN` 时使用 Web API。
2. 否则，同时有 `TWITTER_CONSUMER_KEY` 和 `TWITTER_CONSUMER_SECRET` 时使用 Developer API。
3. 两种都没有时才报 `Twitter API is not configured`。

源码依据：[API 选择逻辑第 10–11、42–45 行](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/twitter/api/index.ts#L10-L11)。若三者全部提供，当前代码会优先使用 Web API，因此 Consumer Key/Secret 不会被这条路径使用：[同文件第 42–45 行](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/twitter/api/index.ts#L42-L45)。

Developer API 还支持可选的 `TWITTER_ACCESS_TOKEN` + `TWITTER_ACCESS_SECRET` 来进行用户身份认证；不提供时 RSSHub 会用 App-only 登录，仅访问公开信息：[RSSHub 说明](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/twitter/namespace.ts#L53-L54)、[实现](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/twitter/api/developer-api/api.ts#L24-L72)。

## 本项目 Compose 核对

`docker-compose.yml` 把三个变量都传给容器本身没有问题，因为每项都允许为空；问题是注释和 README 把它们描述成“X 路由需要三者全部配置”，这与当前 RSSHub 运行时逻辑不符。

建议文档改为二选一：

- 简单自建：只配置 `RSSHUB_TWITTER_AUTH_TOKEN`。
- 使用官方付费 Developer API：只配置 `RSSHUB_TWITTER_CONSUMER_KEY` 和 `RSSHUB_TWITTER_CONSUMER_SECRET`。

上游 `user.ts` 的 `requireConfig` 元数据确实把三个字段都列成非 optional，但这与同仓库的认证说明和实际 API 选择代码冲突：[路由元数据](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/twitter/user.ts#L18-L49)。部署判断应以运行时代码为准。
