---
status: active
when_to_read: 用 upload_group_file / 任何向 NapCat 传本地文件的接口前，或排查「识别URL失败」时
applies_to: [pack, napcat, upload_group_file, onebot, docker]
last_updated: 2026-06-15
---

# pack 上传群文件报「识别URL失败」——NapCat 看不到 bot 的本地路径

## 现象

`pack` 打包后上传群文件失败，NapCat 端日志：

```
[error] 月灵 | 发生错误 Error: 识别URL失败, uri= data/tmp/pack_680653092_20260615_112838.zip
```

bot 把 zip 写到 `data/tmp/...` 再把该路径作为 `upload_group_file` 的 `file` 传过去。

## 根因

`upload_group_file` 的 `file` 字段是**由 NapCat 在它自己的文件系统上读取**，不是 bot 上传字节。看 NapCat 源 `packages/napcat-onebot/action/go-cqhttp/UploadGroupFile.ts`：

```ts
let file = payload.file;
if (fs.existsSync(file)) { file = `file://${file}`; }   // 检查的是 NapCat 自己的盘
const downloadResult = await uriToLocalFile(this.core.NapCatTempPath, file);
```

`uriToLocalFile`（`packages/napcat-common/src/file.ts`）只认 `http(s)://` / `base64://` / `file://`，其余落到默认分支返回 `识别URL失败, uri= ${uri}`（即日志里那行）。

本项目 `docker-compose.yml` 里 **bot 挂了 `./data:/app/data`，napcat 没挂 `./data`**（只有 `napcat-qq` / `napcat-config` 两个卷）。所以 bot 写的 `data/tmp/...` 在 napcat 容器里根本不存在——相对路径还是绝对路径都没用，NapCat 看不到 bot 的盘。

> 误区：以为「改成绝对路径」就能修。绝对路径只在 bot 与 NapCat 共享同一文件系统（同机裸跑或同卷挂载且路径一致）时才有效。独立容器/独立进程部署下无效。

## 处理（已落地）

`plugins/tools/pack.go`：不再写临时文件 + 传路径，改为把 zip 字节以 `base64://` 内联传输，由 NapCat 解码落到它自己的临时目录再上传：

```go
fileURI := "base64://" + base64.StdEncoding.EncodeToString(zipBytes)
ctx.UploadGroupFile(ctx.GroupID(), fileURI, name, "")
```

`base64://` 是 `FileUriType.Base64` 分支（file.ts 中 decode 后 `fs.writeFileSync` 到 NapCat 临时目录），与容器拓扑无关，和 `bot/segment.go` 发图片的做法一致。

## 经验

- 给 NapCat 传文件，**默认用 `base64://` 或 `http(s)://`**，别用本地路径——除非确认 bot 与 NapCat 同文件系统且路径在两端一致。
- 代价：`base64://` 把整个文件塞进一条 WS 帧（体积 +33%）。pack 有 `MaxMB` 上限兜底；若以后传超大文件需改走共享卷 + 绝对路径，或 NapCat 的 stream/上传接口。

> 后续（2026-06-18）：这个「代价」真的爆了——大包的 base64:// 单帧撑爆 WS 触发 close 1009 断连 + panic。pack 已从 base64:// 改走 `upload_file_stream` 流式分片。详见 [[2026-06-18-pack-upload-stream-1009]]。本 incident 的根因（NapCat 读自己的盘 / 识别URL失败）仍是有效参考。

相关：[[2026-06-05-napcat-docker-setup]]（容器拓扑 / 卷挂载）、[[2026-06-18-pack-upload-stream-1009]]（base64:// 的体积天花板与流式替代）。
