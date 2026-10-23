package webui

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func imageNotModified(c *gin.Context, version string) bool {
	tag := `W/"` + version + `"`
	c.Header("Cache-Control", "private, no-cache")
	c.Header("ETag", tag)
	for _, candidate := range strings.Split(c.GetHeader("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(tag, "W/") {
			c.Status(http.StatusNotModified)
			return true
		}
	}
	return false
}
