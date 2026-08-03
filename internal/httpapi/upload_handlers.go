package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var allowedIconExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".ico": true,
}

func (h *Handler) uploadIcon(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择图标文件"})
		return
	}
	if file.Size > h.cfg.Upload.MaxBytes() {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("图标大小不能超过 %dMB", h.cfg.Upload.MaxSizeMB)})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedIconExt[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 png/jpg/jpeg/gif/webp/svg/ico 格式"})
		return
	}
	dir := h.cfg.Upload.AbsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filename := fmt.Sprintf("%s%s", uuid.NewString(), ext)
	dst := filepath.Join(dir, filename)
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = time.Now()
	h.writeAudit(c, "upload_icon", "success", filename)
	c.JSON(http.StatusOK, gin.H{
		"icon_path": filename,
		"icon_url":  h.cfg.Upload.URLPrefix + "/" + filename,
	})
}
