package webui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestGallerySortsBeforePaginationAndRenames(t *testing.T) {
	s, root := galleryServer(t)
	s.images = imageassets.NewLibrary(root, []imageassets.Category{{Name: "语录", Upload: true, NameRequired: true, Naming: "quotation"}})
	dir := filepath.Join(root, "images", "语录")
	if err := os.Remove(filepath.Join(dir, "小明.png")); err != nil {
		t.Fatal(err)
	}
	for i := range 22 {
		name := fmt.Sprintf("123_名字%02d_abcd.png", i)
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		mtime := time.Unix(1021-int64(i), 0)
		if err := os.Chtimes(file, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	cookie := login(t, s)
	for _, scenario := range []struct{ sort, first string }{{"modified_desc", "00"}, {"modified_asc", "21"}, {"name_desc", "21"}, {"name_asc", "00"}} {
		rec := imageRequest(s, "GET", "/api/webui/image-assets?category=语录&sort="+scenario.sort, cookie)
		var page imageassets.Page
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != 200 || len(page.Assets) != 20 || !strings.Contains(page.Assets[0].Name, "名字"+scenario.first) {
			t.Fatalf("sort %s: %d %s (%v)", scenario.sort, rec.Code, rec.Body.String(), err)
		}
	}
	rec := imageRequest(s, "GET", "/api/webui/image-assets?category=语录&sort=modified_desc&page=2", cookie)
	var page imageassets.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || len(page.Assets) != 2 || !strings.Contains(page.Assets[0].Name, "名字20") || page.Assets[0].ModifiedAt.IsZero() {
		t.Fatalf("page 2: %s (%v)", rec.Body.String(), err)
	}
	base := "/api/webui/image-assets/" + page.Assets[0].ID
	rename := func(name string, authenticated bool) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"name": name})
		req := httptest.NewRequest("POST", base+"/rename?category=语录", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			req.AddCookie(cookie)
		}
		return serve(s, req)
	}
	if rec := rename("新名字", false); rec.Code != 401 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	for _, name := range []string{"", "../bad", "a/b"} {
		if rec := rename(name, true); rec.Code != 400 {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	if rec := rename("新名字", true); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	files, _ := filepath.Glob(filepath.Join(dir, "123_新名字_*.png"))
	trash, _ := filepath.Glob(filepath.Join(root, "trash", "images", "语录", "*", page.Assets[0].Name))
	if len(files) != 1 || len(trash) != 0 {
		t.Fatalf("rename lost ownership or recycled the original: %v %v", files, trash)
	}
	if rec := rename("再次改名", true); rec.Code != 404 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec := imageRequest(s, "GET", "/api/webui/image-assets?category=语录&sort=invalid", cookie); rec.Code != 400 {
		t.Fatal(rec.Code, rec.Body.String())
	}
}
