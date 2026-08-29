# NapCat 大文件使用分片上传和独立超时

大文件不要通过单条 `base64://` WebSocket 消息发送。base64 会增加约三分之一体积，容易超过消息上限并触发 Close 1009。

项目使用 `upload_file_stream`：

1. 以 256 KiB 为一片发送 `stream_id`、`chunk_data`、`chunk_index`、`total_chunks`、`file_size`、`expected_sha256` 和文件名。
2. 所有分片完成后发送 `is_complete: true`，取得 NapCat 本地的 `file_path`。
3. 再调用 `upload_group_file`；此时路径由 NapCat 自己创建并可读取。

大文件上传使用 `uploadCallTimeout=180s`，普通 OneBot 调用仍使用较短默认超时。不要把外部上传耗时套进通用 10 秒限制，否则文件可能最终成功而 Bot 已误报失败。

连接生命周期使用每连接 `done` channel 通知 `sendLoop` 和在途调用退出，不由接收端关闭仍可能有生产者写入的 `sendCh`。这样断线会返回连接错误，不会出现 `send on closed channel` panic。

相关守护测试覆盖分片元数据、完成请求、连接关闭和自定义响应超时。
