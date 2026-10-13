# RSSHub 路由修补与版本锁定

## 怎么回事

2026-09-08 排查发现：RSSHub 已取得用户资料和 32 条笔记，却因笔记全文解析失败退回匿名抓取，最终报“未返回用户数据”，掩盖了真实错误。另外，旧 Cookie 在服务器上会跳转登录页，通过服务器扫码登录后恢复。

补丁改为逐条获取全文，首次失败后使用已有的标题、封面和原文链接；保留真实错误，并兼容数据包装格式。摘要保留卡片的真实发布时间，让 Bot 按时间排序，避免置顶旧帖挡住新内容。摘要回退不保证完整正文。部署后验证 HTTP 200、32 条笔记，Bot 容器访问正常。

上游参考：[#19505](https://github.com/DIYgod/RSSHub/issues/19505)、[#22740](https://github.com/DIYgod/RSSHub/pull/22740)。原镜像已经包含 #22740，不能仅靠重复升级解决。

## 当前锁定方式

- `Dockerfile` 将基础镜像固定到 digest `sha256:3c5f0b37a54223c4735b5d6428d2886d97ae24e04297a619fb211a2bc9e174aa`。
- Compose 使用本地修复镜像 `yueling-rsshub:threads-fix-v1`，设置 `pull_policy: never`，不会自动拉取上游新版。
- 现有服务器还有同名镜像的 `docker-compose.override.yml`，保留即可。重启、重建不会丢失补丁。
- Cookie 仅保存在服务器 `.env.rsshub`，不进入代码或镜像；Cookie 过期仍需更新。

首次部署或需要重建时，在项目根目录执行：

```bash
docker compose build rsshub
docker compose up -d --no-deps rsshub
```

## 以后自己升级

1. 修改本目录 `Dockerfile` 中的基础 digest；使用新的修复镜像标签，并同步 Compose 和服务器 override。
2. 运行 `node --test deploy/rsshub/*.test.mjs`，再构建镜像。补丁遇到不匹配的上游代码会中止构建，需要重新适配。
3. 验证真实小红书订阅和其他使用中的路由，再保留新版本。服务 `healthy` 不代表抓取成功。

服务器保留原镜像 `yueling-rsshub:before-xhs-fix`；回滚可将 override 的 image 改为该值，再执行 `docker compose up -d --no-deps rsshub`。原镜像不含本补丁，小红书问题可能重现。

## X 串文主帖

X 用户时间线会把主帖放入 `profile-conversation-*` 模块。`twitter-patch.mjs` 展开模块，保留目标作者的非回复主帖，并按推文 ID 去重；转推仍由路由原有 `includeRts` 开关处理。解析后的缓存使用新名称，避免复用缺少串文的条目集合；路由缓存仍可能短暂保留旧输出。

`twitter-parser.fixture.mjs` 保存修复前构建产物中的解析器与调用片段，供回归测试使用。测试覆盖普通推文、Day 8 串文主帖、自回复、他人内容和重复条目。可用 `RSSHUB_TEST_BUNDLE=/path/to/original/twitter-api.mjs node --test deploy/rsshub/*.test.mjs` 对未打补丁的实际构建产物重复验证。上游函数形态变化时补丁中止，不能直接忽略错误。

本补丁不修改 Bot 游标或补发历史消息。已经越过游标的旧主帖即使重新出现在 RSS 中，也需要单独安排补发。线上发布目录 `/projects/yueling-go/releases/rss-threads-20261013/` 保存本次构建文件、Compose 备份及 `rollback.sh`，回滚只重建 RSSHub。
