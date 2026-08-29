# 嵌套合并转发优先读取内联内容

解析 NapCat 合并转发中的 `forward` 段时，先递归读取 `data.content`，只有没有内联内容时才用 `data.id` 调 `get_forward_msg`。

顶层转发通常只有可查询的 `id`；内层转发则常把子消息直接放在 `content` 中，并且没有可二次查询的 `id`。只支持 `id` 会静默漏掉内层图片。

`collectImages` 的稳定顺序是：

```go
inner := parseForwardContent(d.Content)
if len(inner) == 0 && d.ID != "" && !visited[d.ID] {
    visited[d.ID] = true
    if fwd, err := getForward(d.ID); err == nil {
        inner = fwd
    }
}
for _, message := range inner {
    collectImages(message, ..., depth+1, ...)
}
```

`parseForwardContent` 同时接受段位于 `message`、`content` 或 `data` 包装下的形状；保留 `visited` 去重和最大递归深度。`TestCollectImagesInlineForward` 覆盖一层与多层内联转发。
