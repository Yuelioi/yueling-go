package bot

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
)

func TestUploadFileStream(t *testing.T) {
	data := bytes.Repeat([]byte("yueling"), 50*1024) // 350KB → 3 个 128KB 分片
	const chunk = 128 * 1024
	wantSHA := func() string { s := sha256.Sum256(data); return hex.EncodeToString(s[:]) }()

	var chunks []map[string]any
	var completed bool
	call := func(action string, params any) (json.RawMessage, error) {
		if action != "upload_file_stream" {
			t.Fatalf("意外 action %q", action)
		}
		p := params.(map[string]any)
		if ic, _ := p["is_complete"].(bool); ic {
			completed = true
			return json.RawMessage(`{"status":"file_complete","file_path":"/np/cache/x.zip","file_size":358400}`), nil
		}
		chunks = append(chunks, p)
		return json.RawMessage(`{"received_chunks":1}`), nil
	}

	path, err := uploadFileStream(call, data, "x.zip", chunk)
	if err != nil {
		t.Fatalf("uploadFileStream: %v", err)
	}
	if path != "/np/cache/x.zip" {
		t.Fatalf("file_path = %q", path)
	}
	if !completed {
		t.Fatal("未发送 is_complete 完成信号")
	}
	if len(chunks) != 3 {
		t.Fatalf("分片数 = %d, want 3", len(chunks))
	}

	// 校验分片连续性、总数、sha、file_size、同一 stream_id，且拼回原数据
	var reassembled []byte
	streamID := chunks[0]["stream_id"]
	for i, c := range chunks {
		if c["chunk_index"] != i {
			t.Fatalf("chunk[%d] index = %v", i, c["chunk_index"])
		}
		if c["total_chunks"] != 3 {
			t.Fatalf("chunk[%d] total = %v", i, c["total_chunks"])
		}
		if c["file_size"] != len(data) {
			t.Fatalf("chunk[%d] file_size = %v", i, c["file_size"])
		}
		if c["expected_sha256"] != wantSHA {
			t.Fatalf("chunk[%d] sha mismatch", i)
		}
		if c["stream_id"] != streamID {
			t.Fatalf("chunk[%d] stream_id 不一致", i)
		}
		dec, err := base64.StdEncoding.DecodeString(c["chunk_data"].(string))
		if err != nil {
			t.Fatalf("chunk[%d] base64: %v", i, err)
		}
		reassembled = append(reassembled, dec...)
	}
	if !bytes.Equal(reassembled, data) {
		t.Fatal("分片拼回的数据与原始不一致")
	}
}

func TestUploadFileStreamEmpty(t *testing.T) {
	call := func(string, any) (json.RawMessage, error) { t.Fatal("空数据不应发起任何调用"); return nil, nil }
	if _, err := uploadFileStream(call, nil, "x.zip", 1024); err == nil {
		t.Fatal("空数据应返回错误")
	}
}

func TestUploadFileStreamNoFilePath(t *testing.T) {
	// 完成响应缺 file_path（未真正合并）→ 必须报错，不能静默返回空路径
	call := func(action string, params any) (json.RawMessage, error) {
		if ic, _ := params.(map[string]any)["is_complete"].(bool); ic {
			return json.RawMessage(`{"status":"failed"}`), nil
		}
		return json.RawMessage(`{}`), nil
	}
	if _, err := uploadFileStream(call, []byte("hello"), "x.zip", 1024); err == nil {
		t.Fatal("缺 file_path 应返回错误")
	}
}

func TestUploadFileStreamChunkError(t *testing.T) {
	boom := errors.New("ws closed")
	call := func(string, any) (json.RawMessage, error) { return nil, boom }
	if _, err := uploadFileStream(call, []byte("hello"), "x.zip", 2); err == nil {
		t.Fatal("分片失败应返回错误")
	}
}
