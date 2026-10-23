package webui

import (
	"net/http"
	"strconv"

	"github.com/Yuelioi/yueling-go/services/logx"
	"github.com/Yuelioi/yueling-go/services/wuxia"
	"github.com/gin-gonic/gin"
)

func (s *Server) wuxiaScope(c *gin.Context) (int64, bool) {
	if s.wuxia == nil {
		jsonError(c, http.StatusServiceUnavailable, "江湖插件尚未初始化")
		return 0, false
	}
	id, err := strconv.ParseInt(c.Param("scope"), 10, 64)
	if err != nil || id < 0 {
		jsonError(c, http.StatusBadRequest, "群号无效，0代表全局配置")
		return 0, false
	}
	return id, true
}
func (s *Server) handleWuxiaGet(c *gin.Context) {
	group, ok := s.wuxiaScope(c)
	if !ok {
		return
	}
	cfg, err := s.wuxia.Configuration(c.Request.Context(), group)
	if err != nil {
		s.wuxiaFailure(c, err)
		return
	}
	players := []wuxia.View{}
	ledger := []wuxia.LedgerEntry{}
	if group > 0 {
		players, err = s.wuxia.Ranking(c.Request.Context(), group, 100)
		if err != nil {
			s.wuxiaFailure(c, err)
			return
		}
		ledger, err = s.wuxia.Ledger(c.Request.Context(), group, 0)
		if err != nil {
			s.wuxiaFailure(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "config": cfg, "catalog": s.wuxia.Catalog(), "players": players, "ledger": ledger})
}
func (s *Server) handleWuxiaPut(c *gin.Context) {
	group, ok := s.wuxiaScope(c)
	if !ok {
		return
	}
	var settings wuxia.Settings
	if err := c.ShouldBindJSON(&settings); err != nil {
		jsonError(c, http.StatusBadRequest, "数值格式无效")
		return
	}
	if err := settings.Validate(); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.wuxia.SetSettings(c.Request.Context(), group, settings); err != nil {
		s.wuxiaFailure(c, err)
		return
	}
	s.handleWuxiaGet(c)
}
func (s *Server) handleWuxiaReset(c *gin.Context) {
	group, ok := s.wuxiaScope(c)
	if !ok {
		return
	}
	if group == 0 {
		jsonError(c, http.StatusBadRequest, "全局默认不能恢复群继承")
		return
	}
	if err := s.wuxia.ResetSettings(c.Request.Context(), group); err != nil {
		s.wuxiaFailure(c, err)
		return
	}
	s.handleWuxiaGet(c)
}
func (s *Server) wuxiaFailure(c *gin.Context, err error) {
	logx.Errorf("[webui] wuxia: %v", err)
	jsonError(c, http.StatusInternalServerError, "江湖数据读取或保存失败")
}
