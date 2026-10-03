package webui

import (
	"encoding/json"
	"io"
	"net/http"

	bservice "github.com/Yuelioi/yueling-go/services/bilibili"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleBilibiliGet(c *gin.Context) {
	if s.bilibili == nil {
		jsonError(c, http.StatusServiceUnavailable, "B站服务未初始化")
		return
	}
	settings, err := s.bilibili.Settings()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	contacts := []bservice.Contact{}
	if s.bilibili.AccountID() > 0 {
		contacts, err = s.bilibili.Contacts()
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "settings": settings, "contacts": contacts,
		"status": s.bilibili.Status(), "login_configured": s.bilibili.AccountID() > 0})
}

func (s *Server) handleBilibiliSet(c *gin.Context) {
	if s.bilibili == nil {
		jsonError(c, http.StatusServiceUnavailable, "B站服务未初始化")
		return
	}
	var settings bservice.Settings
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		jsonError(c, http.StatusBadRequest, "B站设置格式无效")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		jsonError(c, http.StatusBadRequest, "B站设置格式无效")
		return
	}
	if err := settings.Validate(); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.bilibili.SaveSettings(settings); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.handleBilibiliGet(c)
}
