# Cockpit — yueling-go

**Last updated**: 2026-06-10 by 月离 (v1.3.0 发布：图片可选转 JPEG 入库+发图两侧 + .gitattributes 统一 LF)
**Active focus**: v1.3.0 已发布（图片可选转 JPEG：添加* 入库按 [image] 开关转、ImageBytes 发图侧治历史大图）；无进行中开发任务

## 进行中

<!-- AUTO:inprogress -->
- [2026-06-15-pack-images.md](specs/2026-06-15-pack-images.md) — 回复一条消息发 pack，把该消息（含递归展开的合并转发）里的所有图片下载打成 zip 上传到群文件；新增独立插件 plugins/tools/pack.go + bot.GetForwardMsg API 封装
- [2026-06-15-pack-images.md](plans/2026-06-15-pack-images.md) — pack 功能实现计划——GetForwardMsg 封装 + 递归抽图 collectImages + zip 打包上传 + 可配置上限([pack] max_images/max_mb，默认 100/100)，6 个 TDD 任务
<!-- /AUTO -->

## 下一步

无进行中任务。入库转换默认关；按需在 `config.toml` 加 `[image] convert=true` 跑张真图验证。main 与 origin 已同步（v1.3.0）。

## Hanging tasks

- (none)
