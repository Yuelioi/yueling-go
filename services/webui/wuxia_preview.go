package webui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Yuelioi/yueling-go/services/wuxia"
	"github.com/Yuelioi/yueling-go/services/wuxia/cards"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleWuxiaPreviewCatalog(c *gin.Context) {
	c.Header("Cache-Control", "private, no-cache")
	c.JSON(http.StatusOK, gin.H{"catalog": wuxia.EquipmentPreviewCatalog(), "balance": wuxia.EquipmentBalanceReport(), "weapon_chances": wuxia.WeaponPreviewChances()})
}

func (s *Server) handleWuxiaPreviewBuild(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	var request struct {
		Left  wuxia.PreviewBuild `json:"left"`
		Right wuxia.PreviewBuild `json:"right"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonError(c, 400, "预览参数格式无效")
		return
	}
	result, err := wuxia.PreviewEquipment(request.Left, request.Right)
	if err != nil {
		jsonError(c, 400, err.Error())
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(200, result)
}

func (s *Server) handleWuxiaPreviewCard(c *gin.Context) {
	var build wuxia.PreviewBuild
	raw := c.Query("build")
	if len(raw) > 4096 || json.Unmarshal([]byte(raw), &build) != nil {
		jsonError(c, 400, "预览参数格式无效")
		return
	}
	catalog := wuxia.EquipmentPreviewCatalog()
	view, err := catalog.PreviewBuild(build)
	if err != nil {
		jsonError(c, 400, err.Error())
		return
	}
	serialized, _ := json.Marshal(view)
	hash := sha256.Sum256(append([]byte(catalog.Version+"-preview-card-1"), serialized...))
	if imageNotModified(c, fmt.Sprintf("%x", hash[:])) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	data, err := cards.Profile(ctx, view, wuxia.Daily{}, 3)
	if err != nil {
		c.Header("Cache-Control", "private, no-store")
		jsonError(c, 500, "预览图片生成失败")
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, "image/png", data)
}

func (s *Server) handleWuxiaPreviewDraw(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var req struct {
		Profession string `json:"profession"`
		Count      int    `json:"count"`
	}
	if c.ShouldBindJSON(&req) != nil {
		jsonError(c, 400, "试抽参数无效")
		return
	}
	var seed [8]byte
	if _, err := rand.Read(seed[:]); err != nil {
		jsonError(c, 500, "随机源不可用")
		return
	}
	items, err := wuxia.PreviewWeaponDraw(req.Profession, req.Count, binary.LittleEndian.Uint64(seed[:]))
	if err != nil {
		jsonError(c, 400, err.Error())
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(200, gin.H{"items": items})
}
