---
status: active
when_to_read: 向 NapCat 传大文件（upload_group_file/群文件）、或排查 close 1009 message too big / send on closed channel panic 时
applies_to: [pack, napcat, upload_group_file, upload_file_stream, onebot, websocket, bot/bot.go, bot/api.go, bot/filestream.go, plugins/tools/pack.go]
last_updated: 2026-06-18
recurrences: 2
resolved_by: bot/filestream.go (UploadFileStream 分片) + bot/bot.go/api.go (done 信号防 panic) + plugins/tools/pack.go + bot/api.go (callT 放宽上传超时 RC-C)
---

# base64:// 上传撑爆 WS(1009) + send-on-closed-channel panic：改 upload_file_stream 流式分片

## Signature
- symptom: |
    WRN [bot] connection closed: websocket: close 1009 (message too big)
    ERR [pack] 上传群文件失败: response timeout: upload_group_file
    ERR [bot] handler panic: send on closed channel
- error_type: websocket close 1009 / panic: send on closed channel
- where: bot sendLoop/recvLoop（连接层）+ pack upload（plugins/tools/pack.go）
- trigger: 回复一条图很多的消息发 pack，base64:// 内联的大 zip 走单条 WS 发给 NapCat

## 症状/复现

回复图多的消息发 `pack` → 日志连着三条：`close 1009 (message too big)` → `upload_group_file` 响应超时 → `handler panic: send on closed channel`，随后 NapCat 重连。小图正常，图一多必崩。

## 根因（两个 bug 串成级联）

**RC-A：base64:// 内联上传撑爆单条 WS 消息上限。**
上一版（见 [[2026-06-15-pack-upload-base64]]）为绕开「NapCat 看不到 bot 本地路径」改成 base64:// 内联，把整个 zip 塞进 `upload_group_file` 的 JSON 走**同一条 WS**。`pack.max_mb` 默认 100MB → base64 再膨胀 33% → 单条消息可达 ~133MB → 超过 NapCat 的 WS 读上限 → NapCat 回 **Close 1009** 并断开。base64:// 解决了路径问题，却把大文件塞进了不该塞的通道。

**RC-B：`call()` 往 sendCh 发送不防连接关闭 → panic。**
`recvLoop` 退出时 `defer close(sendCh)`，但 handler goroutine 还在 `call()` 里 `a.sendCh <- payload`。1009 断线 → recvLoop 关 sendCh → upload 的 call 超时返回 → pack 接着 `ctx.Reply("上传失败")` 再 call → **往已关闭的 channel 发送 → panic**。这是「消费者关闭了生产者仍在写的 channel」的经典错误，任何断线撞上在途 handler 都会 panic，不限于 pack。

## 修法

**RC-B（健壮性，先修，无歧义）：** 引入 per-connection `done chan struct{}`；`recvLoop` 退出 `close(done)` 而**不再 close(sendCh)**；`sendLoop` 和 `call()` 都 select `<-done` 退出/快速失败。任何断线 → 在途 call 返回 `connection closed: <action>`，不再 panic。守护测试 `bot.TestCallOnClosedConnNoPanic`。

**RC-A（根治）：** 改用 NapCat `upload_file_stream` 流式分片（需 NapCat ≥ v4.8.115）：
- 把 zip 切成 256KB 分片，逐片 `upload_file_stream{stream_id, chunk_data(b64), chunk_index, total_chunks, file_size, expected_sha256, filename, file_retention}`——**每片都是普通 echo 响应，现有 `call()` 直接能用**（上传路径不是多帧流式响应，别被文档吓到）。
- 末尾 `upload_file_stream{stream_id, is_complete:true}` → 返回 `data.file_path`（协议端本机落盘路径）。
- 再 `upload_group_file(groupID, file_path, ...)`——路径在 NapCat 自己机器上，必然可读。
- 封装在 `bot.UploadFileStream(data, filename) (path, error)`，pack 调用它取代 base64://。

坑根：**向 NapCat 传大文件不要走单条 WS 消息（base64:// 有体积天花板），用 upload_file_stream 分片。** 协议端来源：NapCat 官方 test_upload_stream.py。

## [Case 2] 手验① 暴露 RC-C：10s call 超时误报「上传失败」（流式上传其实成功）

2026-06-18 重新部署后跑手验①，线上日志确认 **1009 与 panic 已根治**——大包走 upload_file_stream 流式分片，NapCat **不再断连**，小包（1/2 张）干净成功。但大包出现新症状：bot 报「上传失败」，用户反馈「上传了 20s 你却提前说失败了，其实后续上传成功了」。

**RC-C：`call()` 硬编码 10s 响应超时，对大文件上传太短。** 日志 `08:23:58 文件发送中` → `08:24:08 上传失败` 正好 10s——`upload_group_file` 把 ~100MB 推到 QQ 服务器实测可达 ~20s，10s 超时在上传**仍在进行**时返回 `response timeout`，pack 据此回「上传失败」，但 NapCat 随后传完了。误报，非真失败。

**修法：** 拆 `call` 为 `callT(action, params, respTimeout)`；普通调用仍 `defaultCallTimeout=10s`，大文件上传走 `uploadCallTimeout=180s`。`UploadGroupFile`（bot/api_files.go）+ `UploadFileStream` 的分片/`is_complete` 合并（bot/filestream.go）改用 `uploadCallTimeout`。守护测试 `bot.TestCallTHonorsTimeout`（确定性：短超时无响应→response timeout；响应及时→成功；并断言 `uploadCallTimeout > defaultCallTimeout`）。坑根：**把大文件推到外部服务器的 API 调用，响应超时要按上传时长留余量，别用通用 10s。** 待重新部署后手验①复测确认不再误报。

## Cases
- 2026-06-18 首次：用户线上日志 close 1009 + panic。RC-B + 流式上传一并修，加 4 个单测（filestream）+ 1 个连接层测试。
- 2026-06-18 Case 2：手验① 确认 1009/panic 已根治，但暴露 RC-C（10s 超时误报）。已修 + 守护测试，待复测。
