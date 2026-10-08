package webui

import (
	"net/http"
	"strconv"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/gin-gonic/gin"
)

func joinReviewScope(c *gin.Context) (int64, bool) {
	raw := c.Param("scope")
	if raw == "default" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		jsonError(c, http.StatusBadRequest, "无效的群号")
		return 0, false
	}
	return id, true
}

func (s *Server) handleJoinReviewGet(c *gin.Context) {
	id, ok := joinReviewScope(c)
	if !ok {
		return
	}
	state, err := db.GetJoinReview(id)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "读取入群审核失败")
		return
	}
	c.JSON(http.StatusOK, state)
}

func (s *Server) handleJoinReviewSet(c *gin.Context) {
	id, ok := joinReviewScope(c)
	if !ok {
		return
	}
	var cfg db.JoinReviewConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		jsonError(c, http.StatusBadRequest, "无效的入群审核配置")
		return
	}
	cfg, err := db.ValidateJoinReview(id, cfg)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := db.SetJoinReview(id, cfg); err != nil {
		jsonError(c, http.StatusInternalServerError, "保存入群审核失败")
		return
	}
	s.handleJoinReviewGet(c)
}
