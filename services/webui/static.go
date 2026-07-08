package webui

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) mountStatic(r *gin.Engine) {
	dist := filepath.Join("webui", "dist")
	r.NoRoute(func(c *gin.Context) {
		if isAPIPath(c.Request.URL.Path) {
			jsonError(c, http.StatusNotFound, "not found")
			return
		}

		if filePath, ok := staticFilePath(dist, c.Request.URL.Path); ok {
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				c.File(filePath)
				return
			}
		}

		index := filepath.Join(dist, "index.html")
		if _, err := os.Stat(index); err == nil {
			c.File(index)
			return
		}
		c.String(http.StatusNotFound, "webui frontend is not built")
	})
}

func isAPIPath(requestPath string) bool {
	return requestPath == "/api" || strings.HasPrefix(requestPath, "/api/")
}

func staticFilePath(dist, urlPath string) (string, bool) {
	cleaned := path.Clean("/" + urlPath)
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}
	rel = filepath.FromSlash(rel)
	if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" {
		return "", false
	}

	base, err := filepath.Abs(dist)
	if err != nil {
		return "", false
	}
	target, err := filepath.Abs(filepath.Join(base, rel))
	if err != nil {
		return "", false
	}
	if !pathWithin(base, target) {
		return "", false
	}
	return target, true
}

func pathWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return !filepath.IsAbs(rel)
}
