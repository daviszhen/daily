package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smart-daily/internal/model"
)

type FeedbackHandler struct{ db *gorm.DB }

func NewFeedbackHandler(db *gorm.DB) *FeedbackHandler { return &FeedbackHandler{db: db} }

func (h *FeedbackHandler) Submit(c *gin.Context) {
	var req struct{ Content string `json:"content"` }
	if err := c.ShouldBindJSON(&req); err != nil || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content required"})
		return
	}
	fb := model.Feedback{
		MemberID:   c.GetInt("user_id"),
		MemberName: c.GetString("user_name"),
		Content:    req.Content,
	}
	if err := h.db.Create(&fb).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, fb)
}

func (h *FeedbackHandler) List(c *gin.Context) {
	var items []model.Feedback
	q := h.db.Order("created_at DESC")
	if !c.GetBool("is_admin") {
		q = q.Where("member_id = ?", c.GetInt("user_id"))
	}
	q.Find(&items)
	c.JSON(http.StatusOK, items)
}

func (h *FeedbackHandler) Close(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.db.Model(&model.Feedback{}).Where("id = ?", id).Update("status", "closed")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *FeedbackHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	h.db.Where("id = ?", id).Delete(&model.Feedback{})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
