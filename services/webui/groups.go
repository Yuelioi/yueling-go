package webui

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/gin-gonic/gin"
)

func buildGroupMessage(req groupMessageRequest) (bot.Message, error) {
	msg := bot.Msg()
	hasSegment := false

	text := strings.TrimSpace(req.Text)
	if text != "" {
		msg.Text(text)
		hasSegment = true
	}

	for _, userID := range req.AtUserIDs {
		if userID <= 0 {
			return nil, errors.New("invalid at_user_id")
		}
		msg.At(userID)
		hasSegment = true
	}

	for _, image := range req.Images {
		image = strings.TrimSpace(image)
		if image == "" {
			return nil, errors.New("image required")
		}
		msg.Image(image)
		hasSegment = true
	}

	if !hasSegment {
		return nil, errors.New("message required")
	}
	return msg.Build(), nil
}

func (s *Server) handleGroups(c *gin.Context) {
	lister := s.resolveGroupLister()
	if lister == nil {
		jsonError(c, http.StatusServiceUnavailable, "bot not connected")
		return
	}
	groups, err := lister.GetGroupList()
	if err != nil {
		jsonError(c, http.StatusBadGateway, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "groups": groups})
}

func (s *Server) handleSendGroupMessage(c *gin.Context) {
	groupID, ok := parseInt64Param(c, "groupID")
	if !ok {
		return
	}
	var req groupMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	msg, err := buildGroupMessage(req)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	sender := s.resolveGroupSender()
	if sender == nil {
		jsonError(c, http.StatusServiceUnavailable, "bot not connected")
		return
	}
	messageID, err := sender.SendGroupMsg(groupID, msg)
	if err != nil {
		jsonError(c, http.StatusBadGateway, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message_id": messageID})
}
