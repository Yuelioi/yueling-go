# 跨容器向 NapCat 传文件使用可读取的 URI

`upload_group_file.file` 由 NapCat 在它自己的文件系统中读取。Bot 容器里的相对路径或绝对路径，除非两端挂载同一目录且路径一致，否则对 NapCat 不可见。

传输方式按体积选择：

- 小文件可以使用 `base64://` 或 NapCat 能访问的 `http(s)://`。
- 大文件使用 `upload_file_stream` 分片，避免把整个 base64 文件塞进一条 WebSocket 消息。
- 只有明确共享文件系统时才使用 `file://`，并验证 NapCat 看到的路径，而不是 Bot 看到的路径。

项目的 `pack` 大文件路径已经使用流式上传；不要恢复成“先写 Bot 本地临时文件，再把路径传给 NapCat”。
