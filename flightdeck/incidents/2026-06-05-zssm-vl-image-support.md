---
status: active
when_to_read: 配置或排查 zssm 图片识别、选择 VL 模型前
applies_to: [zssm, vl, config]
last_updated: 2026-06-05
---

# zssm 图片识别需多模态 VL 模型

## 现象

zssm 回复图片时返回「图片识别失败」。日志（迁移到 logx 后才看得到）显示底层错误：

```
WRN [zssm] 图片识别失败 url=...: ... 404 No endpoints found that support image input
```

## 根因

`config.toml` 的 `[ai.vl].model` 配成了**纯文本模型**（如 `mimo-v2.5-pro`）。该端点对纯文本请求返回 200，但带 `image_url` 的多模态请求返回 404 `No endpoints found that support image input`。模型本身不支持图片输入，不是代码 bug。

## 排查手段

直接对端点发两次 OpenAI 兼容请求验证边界：
1. 纯文本 `{"model":...,"messages":[{"role":"user","content":"hi"}]}` → 看 key/endpoint/model 是否可用；
2. 多模态（`content` 带 `image_url` data URL）→ 看模型是否支持图片。
两者分别 200 / 404 即可定位到「模型不支持图片」。

## 处理

`[ai.vl].model` 换成支持图片输入的多模态模型（如 `Qwen/Qwen2.5-VL-72B-Instruct`，硅基流动端点）。不配 `[ai.vl].key` 时 zssm 遇到图片直接回「未配置图片识别」，属正常降级。

相关：[[2026-06-05-zssm-go-migration]]、错误可见性来自 [[2026-06-05-logging]] 的 logx 迁移。
