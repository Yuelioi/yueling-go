package webui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Yuelioi/yueling-go/services/runtimeconfig"
	"github.com/gin-gonic/gin"
)

func (s *Server) BindConfiguration(m *runtimeconfig.Manager) { s.configuration = m }

func (s *Server) handleConfigGet(c *gin.Context) {
	if s.configuration == nil {
		jsonError(c, 503, "配置服务未初始化")
		return
	}
	c.JSON(http.StatusOK, s.configuration.View())
}

func (s *Server) handleConfigSet(c *gin.Context) {
	if s.configuration == nil {
		jsonError(c, 503, "配置服务未初始化")
		return
	}
	var request runtimeconfig.Update
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		jsonError(c, 400, "配置格式无效")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		jsonError(c, 400, "配置格式无效")
		return
	}
	reload, err := s.configuration.Update(request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, runtimeconfig.ErrConflict) {
			status = http.StatusConflict
		}
		if errors.Is(err, runtimeconfig.ErrPersistence) {
			status = http.StatusInternalServerError
		}
		jsonError(c, status, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "reload": reload, "config": s.configuration.View()})
}
