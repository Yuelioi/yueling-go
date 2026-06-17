package bot

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// streamChunkSize 是每个分片的原始字节数。base64 后约 1.35x，单条 WS 消息远小于协议端
// 的帧上限——这正是流式上传相对 base64:// 内联的意义：大文件也不会撑爆单条消息（close 1009）。
const streamChunkSize = 256 * 1024

// callFunc 抽象出 BotAPI.call 以便单测注入。
type callFunc func(action string, params any) (json.RawMessage, error)

// UploadFileStream 用 NapCat 的 upload_file_stream 分片上传 data，返回协议端落盘后的文件路径。
// 该路径在协议端本机，可直接交给 upload_group_file 等接口引用（无需 bot 与 NapCat 共享文件系统）。
// 需要 NapCat v4.8.115+。
func (a *BotAPI) UploadFileStream(data []byte, filename string) (string, error) {
	return uploadFileStream(a.call, data, filename, streamChunkSize)
}

func uploadFileStream(call callFunc, data []byte, filename string, chunkSize int) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("upload_file_stream: empty data")
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	streamID := uuid.NewString()
	total := (len(data) + chunkSize - 1) / chunkSize

	for i := range total {
		start := i * chunkSize
		end := min(start+chunkSize, len(data))
		if _, err := call("upload_file_stream", map[string]any{
			"stream_id":       streamID,
			"chunk_data":      base64.StdEncoding.EncodeToString(data[start:end]),
			"chunk_index":     i,
			"total_chunks":    total,
			"file_size":       len(data),
			"expected_sha256": sha,
			"filename":        filename,
			"file_retention":  60000,
		}); err != nil {
			return "", fmt.Errorf("upload_file_stream chunk %d/%d: %w", i, total, err)
		}
	}

	raw, err := call("upload_file_stream", map[string]any{
		"stream_id":   streamID,
		"is_complete": true,
	})
	if err != nil {
		return "", fmt.Errorf("upload_file_stream complete: %w", err)
	}
	var resp struct {
		Status   string `json:"status"`
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("upload_file_stream complete decode: %w", err)
	}
	if resp.FilePath == "" {
		return "", fmt.Errorf("upload_file_stream incomplete: status=%q", resp.Status)
	}
	return resp.FilePath, nil
}
