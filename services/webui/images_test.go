package webui

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yuelioi/yueling-go/services/imageassets"
)

func galleryServer(t *testing.T) (*Server, string) {
	t.Helper()
	s := newTestServer()
	root := t.TempDir()
	dir := filepath.Join(root, "images", "语录")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 800, 400))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "小明.png"), data.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	s.images = imageassets.NewLibrary(root, []imageassets.Category{{Name: "语录"}, {Name: "猫猫", External: true}})
	return s, root
}
func imageRequest(s *Server, method, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return serve(s, req)
}
func TestImageLibraryRequiresAdminSession(t *testing.T) {
	s, _ := galleryServer(t)
	for _, tc := range []struct{ method, path string }{{"GET", "/api/webui/image-assets/categories"}, {"GET", "/api/webui/image-assets?category=语录"}, {"GET", "/api/webui/image-assets/id/preview?category=语录"}, {"DELETE", "/api/webui/image-assets/id?category=语录"}} {
		rec := imageRequest(s, tc.method, tc.path, nil)
		if rec.Code != 401 {
			t.Fatalf("%s: %d", tc.path, rec.Code)
		}
	}
}
func TestImageGalleryListPreviewDelete(t *testing.T) {
	s, root := galleryServer(t)
	cookie := login(t, s)
	rec := imageRequest(s, "GET", "/api/webui/image-assets?category=语录", cookie)
	var page imageassets.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != 200 || len(page.Assets) != 1 {
		t.Fatalf("%d %s %v", rec.Code, rec.Body.String(), err)
	}
	base := "/api/webui/image-assets/" + page.Assets[0].ID
	for _, thumbnail := range []bool{false, true} {
		path := base + "/preview?category=语录"
		if thumbnail {
			path += "&thumbnail=1"
		}
		rec = imageRequest(s, "GET", path, cookie)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if thumbnail && (cfg.Width != 384 || cfg.Height != 192) {
			t.Fatal(cfg)
		}
	}
	rec = imageRequest(s, "DELETE", base+"?category=语录", cookie)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "images", "语录", "小明.png")); !os.IsNotExist(err) {
		t.Fatal("image still present")
	}
	copies, _ := filepath.Glob(filepath.Join(root, "trash", "images", "语录", "*", "小明.png"))
	if len(copies) != 1 {
		t.Fatal("missing trash copy")
	}
	if rec = imageRequest(s, "GET", base+"/preview?category=语录", cookie); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	if rec = imageRequest(s, "DELETE", base+"?category=语录", cookie); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}
func TestGalleryRejectsExternalAndPathRequests(t *testing.T) {
	s, _ := galleryServer(t)
	cookie := login(t, s)
	for _, path := range []string{"/api/webui/image-assets?category=猫猫", "/api/webui/image-assets?category=../语录", "/api/webui/image-assets?category=语录&page=0"} {
		if rec := imageRequest(s, "GET", path, cookie); rec.Code != 400 {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
	rec := imageRequest(s, "DELETE", "/api/webui/image-assets/00000000000000000000000000000000?category=猫猫", cookie)
	if rec.Code != 400 {
		t.Fatal(rec.Code)
	}
}
