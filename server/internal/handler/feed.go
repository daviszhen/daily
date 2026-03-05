package handler

import (
	"net/http"
	"smart-daily/internal/repository"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type FeedHandler struct{ topicRepo *repository.TopicRepo }

func NewFeedHandler(topicRepo *repository.TopicRepo) *FeedHandler {
	return &FeedHandler{topicRepo: topicRepo}
}

// defaultDateRange returns (last Monday, yesterday) as default range.
func defaultDateRange() (string, string) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7
	}
	// Last Monday = this Monday - 7 days
	lastMonday := now.AddDate(0, 0, -(wd - 1 + 7))
	return lastMonday.Format("2006-01-02"), yesterday.Format("2006-01-02")
}

func parseDateRange(c *gin.Context) (string, string) {
	start := c.Query("start")
	end := c.Query("end")
	if start == "" || end == "" {
		start, end = defaultDateRange()
	}
	return start, end
}

// --- 团队动态 ---

// FeedByMember returns daily summaries grouped by member.
// GET /api/feed/by-member?start=&end=
func (h *FeedHandler) FeedByMember(c *gin.Context) {
	start, end := parseDateRange(c)
	rows, err := h.topicRepo.ListSummariesByDateRange(c.Request.Context(), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Group by member
	type memberFeed struct {
		MemberID   int                              `json:"member_id"`
		MemberName string                           `json:"member_name"`
		Items      []repository.MemberDailySummary  `json:"items"`
	}
	memberMap := map[int]*memberFeed{}
	var order []int
	for _, r := range rows {
		if _, ok := memberMap[r.MemberID]; !ok {
			memberMap[r.MemberID] = &memberFeed{MemberID: r.MemberID, MemberName: r.MemberName}
			order = append(order, r.MemberID)
		}
		memberMap[r.MemberID].Items = append(memberMap[r.MemberID].Items, r)
	}
	result := make([]memberFeed, 0, len(order))
	for _, id := range order {
		result = append(result, *memberMap[id])
	}

	c.JSON(http.StatusOK, gin.H{"start": start, "end": end, "members": result})
}

// FeedByTopic returns topic activities grouped by topic.
// GET /api/feed/by-topic?start=&end=
func (h *FeedHandler) FeedByTopic(c *gin.Context) {
	start, end := parseDateRange(c)
	activities, err := h.topicRepo.ListByDateRange(c.Request.Context(), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type topicActivityItem struct {
		MemberName string `json:"member_name"`
		DailyDate  string `json:"daily_date"`
		Content    string `json:"content"`
	}
	type topicFeed struct {
		Topic   string              `json:"topic"`
		Members []string            `json:"members"`
		Items   []topicActivityItem `json:"items"`
	}

	topicMap := map[string]*topicFeed{}
	var topicOrder []string
	for _, a := range activities {
		if _, ok := topicMap[a.Topic]; !ok {
			topicMap[a.Topic] = &topicFeed{Topic: a.Topic}
			topicOrder = append(topicOrder, a.Topic)
		}
		tf := topicMap[a.Topic]
		tf.Items = append(tf.Items, topicActivityItem{
			MemberName: a.MemberName, DailyDate: a.DailyDate, Content: a.Content,
		})
	}
	// Deduplicate members per topic
	for _, tf := range topicMap {
		seen := map[string]bool{}
		for _, item := range tf.Items {
			if !seen[item.MemberName] {
				tf.Members = append(tf.Members, item.MemberName)
				seen[item.MemberName] = true
			}
		}
	}
	result := make([]topicFeed, 0, len(topicOrder))
	for _, t := range topicOrder {
		result = append(result, *topicMap[t])
	}

	c.JSON(http.StatusOK, gin.H{"start": start, "end": end, "topics": result})
}

// --- 数据洞察 ---

// Insights returns topic risk dashboard data.
// GET /api/insights
func (h *FeedHandler) Insights(c *gin.Context) {
	ctx := c.Request.Context()
	insights, err := h.topicRepo.ListInsights(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	risks, _ := h.topicRepo.ListTopicRisks(ctx)

	// Group risks by topic
	riskMap := map[string][]repository.TopicRiskItem{}
	for _, r := range risks {
		riskMap[r.Topic] = append(riskMap[r.Topic], r)
	}

	type insightItem struct {
		repository.TopicInsight
		RiskLevel string                       `json:"risk_level"` // high / medium / low
		Risks     []repository.TopicRiskItem   `json:"risks"`
	}
	result := make([]insightItem, 0, len(insights))
	for _, ins := range insights {
		level := "low"
		if ins.Days > 15 && ins.MemberCnt >= 3 {
			level = "high"
		} else if ins.Days > 7 || ins.MemberCnt >= 3 {
			level = "medium"
		}
		result = append(result, insightItem{
			TopicInsight: ins,
			RiskLevel:    level,
			Risks:        riskMap[ins.Topic],
		})
	}

	c.JSON(http.StatusOK, gin.H{"insights": result})
}

// --- Topic 管理 ---

// ListTopics returns all topics with status.
// GET /api/topics/all
func (h *FeedHandler) ListTopics(c *gin.Context) {
	topics, err := h.topicRepo.ListAllTopics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, topics)
}

// UpdateTopic updates topic name/description.
// PUT /api/topics/:id
func (h *FeedHandler) UpdateTopic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	if err := h.topicRepo.UpdateTopic(c.Request.Context(), id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ResolveTopic marks a topic as resolved.
// PUT /api/topics/:id/resolve
func (h *FeedHandler) ResolveTopic(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.topicRepo.ResolveTopic(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ReopenTopic marks a topic as active again.
// PUT /api/topics/:id/reopen
func (h *FeedHandler) ReopenTopic(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.topicRepo.ReopenTopic(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MergeTopic merges source topic into target.
// POST /api/topics/merge
func (h *FeedHandler) MergeTopic(c *gin.Context) {
	var req struct {
		SourceID   int    `json:"source_id" binding:"required"`
		TargetName string `json:"target_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_id and target_name required"})
		return
	}
	if err := h.topicRepo.MergeTopic(c.Request.Context(), req.SourceID, req.TargetName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
