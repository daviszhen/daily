package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"smart-daily/internal/model"
	"smart-daily/internal/service"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	ai      *service.AIService
	daily   *service.DailyService
	catalog *service.CatalogSync
	pending sync.Map
}

func NewChatHandler(ai *service.AIService, daily *service.DailyService, catalog *service.CatalogSync) *ChatHandler {
	return &ChatHandler{ai: ai, daily: daily, catalog: catalog}
}

func (h *ChatHandler) Chat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Action != "confirm" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "use /api/chat/stream"})
		return
	}

	uid := c.GetInt("user_id")
	val, ok := h.pending.LoadAndDelete(uid)
	if !ok {
		c.JSON(http.StatusOK, model.ChatResponse{Content: "没有待确认的日报，请先输入工作内容。", Type: "text"})
		return
	}
	p := val.(*model.PendingReport)
	slog.Info("chat.confirm", "uid", uid, "member_id", p.MemberID, "date", p.Date, "summary", p.Summary)
	if err := h.daily.Save(c.Request.Context(), p.MemberID, p.Date, p.Content, p.Summary, strings.Join(p.Risks, "; ")); err != nil {
		slog.Error("save daily failed", "err", err)
		c.JSON(http.StatusOK, model.ChatResponse{Content: "保存失败：" + err.Error(), Type: "text"})
		return
	}

	if h.catalog != nil {
		entryID := h.daily.LastInsertID()
		date := p.Date
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		go h.catalog.SyncDailySummary(context.Background(), entryID, p.MemberID, date, p.Content, p.Summary, strings.Join(p.Risks, "; "))
	}

	c.JSON(http.StatusOK, model.ChatResponse{Content: "日报已提交成功！", Type: "text"})
}

type sseWriter struct {
	w http.Flusher
	f gin.ResponseWriter
}

func (s *sseWriter) event(name string, data interface{}) {
	j, _ := json.Marshal(data)
	fmt.Fprintf(s.f, "event: %s\ndata: %s\n\n", name, j)
	s.w.Flush()
}

func (s *sseWriter) token(t string) {
	s.event("token", map[string]string{"token": t})
}

func (s *sseWriter) done() {
	s.event("done", map[string]string{})
}

func (h *ChatHandler) ChatStream(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx := c.Request.Context()
	uid := c.GetInt("user_id")
	name := c.GetString("user_name")
	sse := &sseWriter{w: c.Writer, f: c.Writer}

	switch req.Mode {
	case "report", "supplement":
		slog.Info("chat.stream", "uid", uid, "name", name, "mode", req.Mode, "text", req.Text, "date", req.Date)
		h.streamReport(ctx, sse, uid, req)
	case "query":
		slog.Info("chat.stream", "uid", uid, "name", name, "mode", "query", "question", req.Text)
		h.streamQuery(ctx, sse, req.Text)
	case "summary":
		slog.Info("chat.stream", "uid", uid, "name", name, "mode", "summary")
		h.streamSummary(ctx, sse, uid, name)
	default:
		slog.Info("chat.stream", "uid", uid, "name", name, "mode", "none")
		sse.token("我可以帮您记录日报、查询团队进度或生成总结。请选择下方的功能按钮。")
		sse.done()
	}
}

func (h *ChatHandler) streamReport(ctx context.Context, sse *sseWriter, uid int, req model.ChatRequest) {
	valid, reply, err := h.ai.ValidateWorkContent(ctx, req.Text)
	if err != nil {
		slog.Warn("validate fallback", "err", err)
	}
	if !valid {
		sse.token(reply)
		sse.done()
		return
	}

	summary, err := h.ai.StreamSummarize(ctx, req.Text, sse.token)
	if err != nil {
		slog.Error("stream summarize failed", "err", err)
		sse.done()
		return
	}

	risks, _ := h.ai.DetectRisks(ctx, summary)
	slog.Info("chat.report.done", "uid", uid, "summary", summary, "risks", risks)

	h.pending.Store(uid, &model.PendingReport{
		Content: req.Text, Summary: summary, Risks: risks,
		Date: req.Date, MemberID: uid,
	})

	meta := map[string]interface{}{
		"type":    "summary_confirm",
		"summary": summary,
		"risks":   risks,
	}
	if req.Mode == "supplement" && req.Date != "" {
		meta["isSupplement"] = true
		meta["supplementDate"] = req.Date
	}
	sse.event("result", meta)
	sse.done()
}

func (h *ChatHandler) streamQuery(ctx context.Context, sse *sseWriter, question string) {
	var answer strings.Builder
	var steps []string
	if err := h.ai.StreamQueryAnswer(ctx, question, func(t string) {
		answer.WriteString(t)
		sse.token(t)
	}, func(t string) {
		steps = append(steps, t)
		slog.Info("chat.query.thinking", "question", question, "step", len(steps), "content", t)
		sse.event("thinking", map[string]string{"text": t})
	}); err != nil {
		slog.Error("data asking failed", "err", err)
	}
	slog.Info("chat.query.done", "question", question, "steps", len(steps), "answer", answer.String())
	sse.done()
}

func (h *ChatHandler) streamSummary(ctx context.Context, sse *sseWriter, uid int, name string) {
	data, err := h.daily.GetMemberWeekData(ctx, uid)
	if err != nil {
		slog.Error("get week data failed", "err", err)
	}
	md, err := h.ai.StreamWeeklySummary(ctx, name, data, sse.token)
	if err != nil {
		slog.Error("stream summary failed", "err", err)
		sse.done()
		return
	}

	filename := fmt.Sprintf("周报_%s_%s.md", name, time.Now().Format("20060102"))
	dir := filepath.Join(".", "exports")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, filename), []byte(md), 0644)

	sse.event("meta", map[string]string{
		"downloadUrl":   "/api/files/" + filename,
		"downloadTitle": filename,
	})
	sse.done()
}

func (h *ChatHandler) DownloadFile(c *gin.Context) {
	name := c.Param("name")
	path := filepath.Join(".", "exports", name)
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	c.File(path)
}
