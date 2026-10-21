package webui

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/imageassets"
	"github.com/gin-gonic/gin"
)

func (s *Server) imageLibrary(c *gin.Context) bool {
	c.Header("Cache-Control", "private, no-store")
	if s.images == nil {
		jsonError(c, http.StatusServiceUnavailable, "图片管理尚未初始化")
		return false
	}
	return true
}
func imageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, imageassets.ErrExternal):
		jsonError(c, http.StatusBadRequest, "外部网站图片不支持本地素材管理")
	case errors.Is(err, imageassets.ErrCategory):
		jsonError(c, http.StatusBadRequest, "类目不存在或存在歧义")
	case errors.Is(err, imageassets.ErrAsset):
		jsonError(c, http.StatusNotFound, "图片不存在或已变化，请刷新列表")
	case errors.Is(err, imageassets.ErrUnsafe):
		jsonError(c, http.StatusBadRequest, "无法访问此素材目录或文件")
	case errors.Is(err, imageassets.ErrUpload), errors.Is(err, imageassets.ErrSort), errors.Is(err, imageassets.ErrName), errors.Is(err, imageassets.ErrUnchanged):
		jsonError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, imageassets.ErrConflict):
		jsonError(c, http.StatusConflict, err.Error())
	default:
		jsonError(c, http.StatusInternalServerError, "图片操作失败，请稍后重试")
	}
}
func (s *Server) handleImageCategories(c *gin.Context) {
	if !s.imageLibrary(c) {
		return
	}
	categories := s.images.Categories()
	if categories == nil {
		categories = []imageassets.Category{}
	}
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}
func (s *Server) handleImageAssets(c *gin.Context) {
	if !s.imageLibrary(c) {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 1000000 {
		jsonError(c, http.StatusBadRequest, "页码无效")
		return
	}
	result, err := s.images.ListSorted(c.Request.Context(), c.Query("category"), c.Query("query"), page, c.DefaultQuery("sort", "modified_desc"))
	if err != nil {
		imageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"category": result.Category, "assets": result.Assets, "total": result.Total, "page": result.Number, "page_size": imageassets.PageSize})
}

func (s *Server) handleImageRename(c *gin.Context) {
	if !s.imageLibrary(c) {
		return
	}
	var request struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonError(c, http.StatusBadRequest, "请提供新名称")
		return
	}
	result, err := s.images.Rename(c.Request.Context(), c.Query("category"), c.Param("id"), request.Name)
	if err != nil {
		imageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": result.Destination})
}
func (s *Server) handleImageDelete(c *gin.Context) {
	if !s.imageLibrary(c) {
		return
	}
	name, err := s.images.Delete(c.Request.Context(), c.Query("category"), c.Param("id"))
	if err != nil {
		imageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": name})
}
func (s *Server) handleImagePreview(c *gin.Context) {
	if !s.imageLibrary(c) {
		return
	}
	f, err := s.images.Open(c.Request.Context(), c.Query("category"), c.Param("id"))
	if err != nil {
		imageError(c, err)
		return
	}
	defer f.Close()
	if c.Query("thumbnail") == "1" {
		data, err := imageassets.Thumbnail(c.Request.Context(), f)
		if err != nil {
			jsonError(c, http.StatusUnsupportedMediaType, "图片无法生成预览")
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, "image/png", data)
		return
	}
	header := make([]byte, 512)
	n, err := f.ReadAt(header, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		imageError(c, err)
		return
	}
	mime := http.DetectContentType(header[:n])
	if !strings.HasPrefix(mime, "image/") || strings.Contains(mime, "svg") {
		jsonError(c, http.StatusUnsupportedMediaType, "图片格式不支持预览")
		return
	}
	info, err := f.Stat()
	if err != nil {
		imageError(c, err)
		return
	}
	c.Header("Content-Type", mime)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), f)
}

// The browser queues files individually, bounding request size and retaining
// per-file results when part of a batch fails.
func (s *Server) handleImageUpload(c *gin.Context) {
	if !s.imageLibrary(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, imageassets.MaxBytes+(64<<10))
	err := c.Request.ParseMultipartForm(1 << 20)
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			jsonError(c, 413, "单张图片不得超过 16 MiB")
		} else {
			jsonError(c, 400, "请选择要上传的图片")
		}
		return
	}
	form := c.Request.MultipartForm
	if form == nil || len(form.File) != 1 || len(form.File["file"]) != 1 {
		jsonError(c, 400, "每个上传请求只接受一张图片")
		return
	}
	for key, values := range form.Value {
		if (key != "category" && key != "name" && key != "group_id") || len(values) != 1 || len(values[0]) > 512 {
			jsonError(c, 400, "上传参数无效")
			return
		}
	}
	groupID := int64(0)
	if value := c.PostForm("group_id"); value != "" {
		groupID, err = strconv.ParseInt(value, 10, 64)
		if err != nil || groupID <= 0 {
			jsonError(c, 400, "群号无效")
			return
		}
	}
	file := form.File["file"][0]
	if file.Size > imageassets.MaxBytes {
		jsonError(c, 413, "单张图片不得超过 16 MiB")
		return
	}
	stream, err := file.Open()
	if err != nil {
		jsonError(c, 400, "无法读取上传图片")
		return
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, imageassets.MaxBytes+1))
	if err != nil {
		jsonError(c, 400, "无法读取上传图片")
		return
	}
	if len(data) > imageassets.MaxBytes {
		jsonError(c, 413, "单张图片不得超过 16 MiB")
		return
	}
	cfg := config.Current().Image
	result, err := s.images.Upload(c.Request.Context(), c.PostForm("category"), c.PostForm("name"), groupID, data, imageassets.Options{Convert: cfg.Convert, MinBytes: cfg.ConvertMinKB * 1024, Quality: cfg.ConvertQuality})
	if err != nil {
		imageError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
