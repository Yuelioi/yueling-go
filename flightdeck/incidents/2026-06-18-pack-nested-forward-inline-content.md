---
status: active
when_to_read: 解析合并转发（get_forward_msg / forward 段）取内容前，或排查嵌套转发只取到第一层的问题时
applies_to: [pack, napcat, forward, get_forward_msg, onebot, plugins/tools/pack.go]
last_updated: 2026-06-18
resolved_by: plugins/tools/pack.go (collectImages 优先吃 forward.data.content + parseForwardContent)
---

# 嵌套合并转发取图：内层 forward 无可二次查询 id，子消息内联在 data.content

## Signature
- symptom: 对「合并转发里又套合并转发」的消息发 `pack`，只打包到最外层/第一层的图，内层转发里的图全部丢失（无报错，静默漏图）
- error_type: —
- where: collectImages — plugins/tools/pack.go
- trigger: 回复一条嵌套合并转发消息触发 pack（forward 里套 forward）

## 症状/复现

回复一条「合并转发 A，A 里有一条合并转发 B」的消息发 `pack`：
zip 里只有 A 顶层那几张图，B 内部的图一张都没进来。单层合并转发正常，一旦再嵌套就漏。

## 根因

`collectImages` 处理 `forward` 段时**只走 `data.id` → `GetForwardMsg(id)`** 一条路。

NapCat 的嵌套合并转发不是这个形状：当你对外层 forward 调 `get_forward_msg`，返回的
子消息里那条「内层 forward」段，**子消息是内联在 `data.content` 里的，且自身没有可二次
查询的 `id`**（`d.ID == ""`）。于是 `if d.ID != ""` 判断直接把整条内层分支跳过 → 内层图丢失。

旁证：`bot/parseForwardMsg` 早就要对 `message` / `content` 两种键容错（见
`bot/forward_test.go`），说明 NapCat 确实用 `content` 承载转发内容。

## 修法

`collectImages` 的 `forward` 分支改为**先吃内联 `data.content`，没有内联才回退 `getForward(id)`**：

```go
inner := parseForwardContent(d.Content)
if len(inner) == 0 && d.ID != "" && !visited[d.ID] {
    visited[d.ID] = true
    if fwd, err := getForward(d.ID); err == nil { inner = fwd }
}
for _, im := range inner { collectImages(im, ..., depth+1, ...) }
```

`parseForwardContent` 对节点形状容错（段在 `message` / `content`，或裹在 `data` 下都认），
解不出返回 nil 触发 id 回退。顶层 forward（消息里只带 `{"id":...}`）仍走 getForward，行为不变。
`depth ≤ 5` 继续兜底递归深度。

坑根：**别假设「转发段一定能用 id 二次查询」**——嵌套层的内容是内联的，必须直接读 content。

## Cases
- 2026-06-18 首次：用户报「转发里面还有转发就拾取不到」。加 TestCollectImagesInlineForward（一/两层内联）复现并守护。
