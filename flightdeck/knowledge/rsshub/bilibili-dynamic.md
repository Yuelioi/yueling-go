---
kind: note
summary: "RSSHub B 站 UP 主动态路由、Cookie、风控与容器网络的排查结论。"
activation: reference
read_when: "配置或排查 RSSHub 的 B 站 UP 主动态订阅时。"
---

# RSSHub B 站 UP 主动态

核对日期：2026-08-18。RSSHub 上游基线：`697421be62613f3d1db960f53adb8cd569343a9c`。

## 直接结论

- 当前路由应写成 `/bilibili/user/dynamic/<目标UP主UID>/embed=0`。`disableEmbed=1` 是旧参数，当前实现只读取 `embed` 等 `routeParams`，因此旧参数会被忽略，但它本身不会造成超时。[路由参数声明与示例](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/bilibili/dynamic.ts#L18-L45) [参数解析实现](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/bilibili/dynamic.ts#L264-L274)
- `BILIBILI_COOKIE_*` 在元数据中是可选项，但不配时 RSSHub 必须借助 Playwright 生成临时 Cookie。本项目目前使用普通 `ghcr.io/diygod/rsshub:latest`，又没有 Browserless/Playwright endpoint，因此对本项目而言，配置完整 Cookie 是简单且稳定的方案。[Cookie 回退实现](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/bilibili/cache.ts#L23-L70) [RSSHub 官方 Compose 的两种 Playwright 方案](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/docker-compose.yml#L1-L35)
- 动态路由需要完整 Cookie，只有 `SESSDATA` 不够。RSSHub 路由源码给出的获取方法是：登录 B 站后打开 `https://api.vc.bilibili.com/dynamic_svr/v1/dynamic_svr/dynamic_new?uid=0&type=8`，打开开发者工具的 Network，刷新，选中 `dynamic_new` 请求，复制 Request Headers 里的整个 `Cookie`。[RSSHub 官方获取说明](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/bilibili/dynamic.ts#L36-L45)
- Cookie 是登录凭证，不要写入 `docker-compose.yml`、Git、日志或聊天；只在部署环境的密钥变量中保存。B 站隐私政策也把 Cookie 和登录信息列为需要保护的个人信息：[哔哩哔哩开放平台隐私政策](https://open.bilibili.com/agreement/privacy-policy)。

## 自建容器推荐配置

使用 Cookie 时继续用普通 RSSHub 镜像即可。环境变量名中的 UID 是**提供 Cookie 的登录账号 UID**，不是被订阅的 UP 主 UID：

```yaml
services:
  rsshub:
    image: ghcr.io/diygod/rsshub:latest
    environment:
      BILIBILI_COOKIE_你的登录账号UID: "${RSSHUB_BILIBILI_COOKIE}"
      REQUEST_TIMEOUT: "45000"
      REQUEST_RETRY: "2"
```

部署环境中保存：

```dotenv
RSSHUB_BILIBILI_COOKIE=从浏览器请求头复制的完整一行 Cookie
```

RSSHub 会读取所有以 `BILIBILI_COOKIE_` 开头的变量；动态路由从配置池中取一个 Cookie，因此多个账号也可以组成 Cookie 池。[环境变量解析](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/config.ts#L734-L743) [Cookie 池选择](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/bilibili/cache.ts#L23-L43)

如果明确不想提供登录 Cookie，则必须改用 `ghcr.io/diygod/rsshub:chromium-bundled`，或按 RSSHub 官方 Compose 增加 Browserless 并设置 `PLAYWRIGHT_WS_ENDPOINT`。这条路径占用更多资源，而且 B 站仍可能对临时访客 Cookie 触发风控，不如完整登录 Cookie 稳定。

## 超时、403、412 与 `-352`

需要先区分故障发生在哪一跳：

1. `Get "https://rsshub.app/bilibili/...": context deadline exceeded ... awaiting headers` 表示 Bot 请求公共 `rsshub.app` 时连响应头都没收到。这是 **Bot → 公共 RSSHub** 的连接超时，不是 B 站返回的认证错误。启用自建容器后，Bot 应使用 `http://rsshub:1200`。
2. 自建 RSSHub 日志若显示请求 `api.bilibili.com` 超时，才是 **RSSHub 容器 → B 站** 的 DNS、网络或出口问题。增加 Cookie 不能修复纯连接超时，需从容器内测试 B 站 API 的连通性。
3. HTTP 403/412 表示上游已经响应但拒绝了请求，通常按源站访问控制或风控排查：更新完整 Cookie、降低抓取频率、等待风控解除，并检查出口 IP。它不是“超时太短”，单纯调大 `REQUEST_TIMEOUT` 无效。B 站没有公开这条 Web API 的 403/412 精确语义，因此不能仅凭状态码断言唯一原因。
4. B 站 JSON `code: -352` 是当前 RSSHub 明确识别的风控分支：它会尝试换临时 Cookie 重试，仍为 `-352` 时抛出“遇到源站风控校验，请稍后再试”。[动态请求与风控处理](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/routes/bilibili/dynamic.ts#L276-L301)

## 大陆 Docker 与代理

RSSHub 源码直接请求 `api.bilibili.com` 和 `space.bilibili.com`，没有要求大陆部署必须使用代理。由此推断，在中国大陆部署时应先让 B 站直连；境外代理出口反而可能改变常用登录地域或触发 IP 风控。只有从 RSSHub 容器实测确实无法连接 B 站时，才为 B 站增加代理。

如果同一个 RSSHub 容器还要抓 X，推荐只让 X 走代理、B 站保持直连。RSSHub 读取的是自己的 `PROXY_URI` / `PROXY_URL_REGEX`，不能只依赖 Bot 的 `tools.proxy` 或通用 `HTTP_PROXY`：

```yaml
environment:
  PROXY_URI: "http://host.docker.internal:代理端口"
  PROXY_STRATEGY: "all"
  PROXY_URL_REGEX: '^https?://([^/]+\\.)?(x\\.com|twitter\\.com|twimg\\.com)(/|$)'
```

`host.docker.internal` 适用于 Docker Desktop；Linux Docker 需要把宿主机网关显式加入容器。RSSHub 的代理变量及 URL 正则默认值见[配置源码](https://github.com/DIYgod/RSSHub/blob/697421be62613f3d1db960f53adb8cd569343a9c/lib/config.ts#L795-L816)。

## 最小验证

```powershell
docker compose up -d --force-recreate rsshub
docker compose exec rsshub curl -I --max-time 15 https://api.bilibili.com/x/web-interface/nav
docker compose logs --tail 200 rsshub
```

随后从 Compose 网络内请求 `/bilibili/user/dynamic/4279370/embed=0`。如果 `/healthz` 正常但 B 站 API 超时，排查容器出口；如果返回 403/412 或日志出现 `-352`，排查 Cookie、频率与出口 IP。
