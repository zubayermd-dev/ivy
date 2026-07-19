package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zubayermd-dev/ivy/internal/model"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	db *gorm.DB
}

func NewWebhookHandler(db *gorm.DB) *WebhookHandler {
	return &WebhookHandler{db: db}
}

func (h *WebhookHandler) ListWebhooks(c *gin.Context) {
	iccid := c.Query("iccid")
	var list []model.Webhook
	if err := h.db.Where("iccid = ?", iccid).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *WebhookHandler) CreateWebhook(c *gin.Context) {
	var wh model.Webhook
	if err := c.ShouldBindJSON(&wh); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate URL - only allow http/https and block internal IPs
	if !strings.HasPrefix(wh.URL, "http://") && !strings.HasPrefix(wh.URL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL must start with http:// or https://"})
		return
	}
	// Block SSRF to internal networks
	if strings.Contains(wh.URL, "127.0.0.1") || strings.Contains(wh.URL, "localhost") ||
		strings.Contains(wh.URL, "169.254.") || strings.Contains(wh.URL, "10.") ||
		strings.Contains(wh.URL, "172.16.") || strings.Contains(wh.URL, "192.168.") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL cannot point to internal networks"})
		return
	}

	if err := h.db.Create(&wh).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, wh)
}

func (h *WebhookHandler) DeleteWebhook(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	if err := h.db.Delete(&model.Webhook{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
