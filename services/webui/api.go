package webui

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/gin-gonic/gin"
)

type groupLister interface {
	GetGroupList() ([]bot.GroupInfo, error)
}

type groupMessageSender interface {
	SendGroupMsg(groupID int64, msg bot.Message) (int32, error)
}

type feedSender interface {
	SendGroupText(groupID int64, text string) error
}

type groupMessageRequest struct {
	Text      string   `json:"text"`
	AtUserIDs []int64  `json:"at_user_ids"`
	Images    []string `json:"images"`
}

func parseInt64Param(c *gin.Context, name string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || v <= 0 {
		jsonError(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return v, true
}

func parseIntParam(c *gin.Context, name string) (int, bool) {
	v, err := strconv.Atoi(c.Param(name))
	if err != nil || v <= 0 {
		jsonError(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return v, true
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || v == 0 {
		jsonError(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return uint(v), true
}

func parseOptionalGroupID(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("group_id"))
	if raw == "" {
		return 0, true
	}
	groupID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || groupID <= 0 {
		jsonError(c, http.StatusBadRequest, "invalid group_id")
		return 0, false
	}
	return groupID, true
}

type chatHistoryDeleteRequest struct {
	BeforeAt *int64 `json:"before_at"`
	All      *bool  `json:"all"`
}
