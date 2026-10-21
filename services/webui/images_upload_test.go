package webui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yuelioi/yueling-go/services/imageassets"
)

func multipartUpload(t *testing.T, s *Server, cookie *http.Cookie, fields map[string]string, data []byte, files int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	for range files {
		part, err := writer.CreateFormFile("file", "not-trusted.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/webui/image-assets/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return serve(s, req)
}
func TestImageUploadAuthenticationValidationAndStorage(t *testing.T) {
	s, root := galleryServer(t)
	s.images = imageassets.NewLibrary(root, []imageassets.Category{{Name: "吃的", Upload: true, NameRequired: true, Naming: "label"}, {Name: "语录", Upload: true, NameRequired: true, Naming: "quotation"}, {Name: "外站", External: true}, {Name: "只读"}})
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"category": "吃的", "name": "米饭"}
	if got := multipartUpload(t, s, nil, fields, b.Bytes(), 1); got.Code != 401 {
		t.Fatal(got.Code)
	}
	cookie := login(t, s)
	for _, tc := range []struct {
		category, name, group string
		data                  []byte
		files, status         int
	}{
		{"吃的", "米饭", "", b.Bytes(), 1, 200}, {"吃的", "米饭", "", b.Bytes(), 1, 200},
		{"语录", "小明", "123", b.Bytes(), 1, 200}, {"语录", "小明", "", b.Bytes(), 1, 400},
		{"语录", "小明", "0", b.Bytes(), 1, 400}, {"语录", "小明", "1.5", b.Bytes(), 1, 400},
		{"吃的", "../a", "", b.Bytes(), 1, 400}, {"外站", "a", "", b.Bytes(), 1, 400}, {"只读", "a", "", b.Bytes(), 1, 400},
		{"吃的", "", "", b.Bytes(), 1, 400}, {"吃的", "a", "", []byte("invalid"), 1, 400},
		{"吃的", "a", "", b.Bytes(), 0, 400}, {"吃的", "a", "", b.Bytes(), 2, 400},
		{"吃的", "a", "", make([]byte, imageassets.MaxBytes+1), 1, 413},
		{"吃的", "a", "", make([]byte, imageassets.MaxBytes+(128<<10)), 1, 413},
	} {
		t.Run(fmt.Sprintf("%s/%s/%s/%d/%d", tc.category, tc.name, tc.group, tc.files, len(tc.data)), func(t *testing.T) {
			fields := map[string]string{"category": tc.category, "name": tc.name}
			if tc.group != "" {
				fields["group_id"] = tc.group
			}
			got := multipartUpload(t, s, cookie, fields, tc.data, tc.files)
			if got.Code != tc.status {
				t.Fatalf("%d %s", got.Code, got.Body.String())
			}
			if tc.status == 200 {
				var result imageassets.UploadResult
				if err := json.Unmarshal(got.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if filepath.Ext(result.Name) != ".png" {
					t.Fatal("trusted client extension", result)
				}
			}
		})
	}
	entries, _ := os.ReadDir(filepath.Join(root, "images", "吃的"))
	if len(entries) != 1 {
		t.Fatal("duplicate or invalid requests wrote images", entries)
	}
}
