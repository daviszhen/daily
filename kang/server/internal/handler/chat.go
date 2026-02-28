package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"smart-daily/internal/logger"
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
	session *service.SessionService
	pending sync.Map
}

func NewChatHandler(ai *service.AIService, daily *service.DailyService, catalog *service.CatalogSync) *ChatHandler {
	return &ChatHandler{ai: ai, daily: daily, catalog: catalog}
}

func (h *ChatHandler) SetSessionService(s *service.SessionService) { h.session = s }

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
	ctx := c.Request.Context()
	date := p.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	logger.Info("chat.confirm", "uid", uid, "member_id", p.MemberID, "date", date, "summary", p.Summary)

	risk := strings.Join(p.Risks, "; ")
	entryID, err := h.daily.Save(ctx, p.MemberID, date, p.Content, p.Summary, risk)
	if err != nil {
		logger.Error("save daily failed", "err", err)
		c.JSON(http.StatusOK, model.ChatResponse{Content: "保存失败：" + err.Error(), Type: "text"})
		return
	}

	// 取今天已有总结，和本次摘要合并
	mergedSummary := p.Summary
	var existing model.DailySummary
	if err := h.daily.GetDailySummary(ctx, p.MemberID, date, &existing); err == nil && existing.Summary != "" {
		if merged, err := h.ai.MergeDailySummary(ctx, []string{existing.Summary, p.Summary}); err == nil {
			mergedSummary = merged
		} else {
			logger.Warn("merge summary failed, using latest", "err", err)
		}
	}
	if err := h.daily.UpdateDailySummary(ctx, p.MemberID, date, mergedSummary, risk); err != nil {
		logger.Error("update daily summary failed", "err", err)
	}

	if h.catalog != nil {
		h.catalog.SyncDailySummary(ctx, entryID, p.MemberID, date, p.Content, mergedSummary, risk)
	}

	c.JSON(http.StatusOK, model.ChatResponse{Content: "日报已提交成功！", Type: "text"})

	// Save confirm messages to session
	if req.SessionID != nil {
		cfgJSON, _ := json.Marshal(map[string]interface{}{"type": "summary_confirm", "summary": mergedSummary, "risks": p.Risks})
		h.saveMessages(c.GetString("user_name"), req.SessionID, "确认提交", "日报已提交成功！", string(cfgJSON))
	}
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

// saveMessages persists user input + assistant reply to MOI session (fire-and-forget).
func (h *ChatHandler) saveMessages(userName string, sessionID *int64, userText, assistantText, configJSON string) {
	if h.session == nil || sessionID == nil {
		return
	}
	go func() {
		ctx := context.Background()
		h.session.SaveMessage(ctx, userName, *sessionID, "user", userText, "")
		h.session.SaveMessage(ctx, userName, *sessionID, "assistant", assistantText, configJSON)
	}()
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
		logger.Info("chat.stream", "uid", uid, "name", name, "mode", req.Mode, "text", req.Text, "date", req.Date)
		reply, cfg := h.streamReportCapture(ctx, sse, uid, req)
		h.saveMessages(name, req.SessionID, req.Text, reply, cfg)
	case "query":
		logger.Info("chat.stream", "uid", uid, "name", name, "mode", "query", "question", req.Text)
		question := injectUserIdentity(req.Text, name)
		reply, cfg := h.streamQueryCapture(ctx, sse, question, req)
		h.saveMessages(name, req.SessionID, req.Text, reply, cfg)
	case "summary":
		logger.Info("chat.stream", "uid", uid, "name", name, "mode", "summary")
		h.streamSummary(ctx, sse, uid, name, req.Text)
	default:
		// 智能路由：自动识别意图（带对话历史）
		history := buildHistory(req, 5)
		intent, _ := h.ai.ClassifyIntent(ctx, req.Text, history)
		logger.Info("chat.stream", "uid", uid, "name", name, "mode", "auto", "intent", intent, "text", req.Text)
		switch intent {
		case "report":
			req.Mode = "report"
			reply, cfg := h.streamReportCapture(ctx, sse, uid, req)
			h.saveMessages(name, req.SessionID, req.Text, reply, cfg)
		case "query":
			question := injectUserIdentity(req.Text, name)
			reply, cfg := h.streamQueryCapture(ctx, sse, question, req)
			h.saveMessages(name, req.SessionID, req.Text, reply, cfg)
		default:
			var reply strings.Builder
			history := buildHistory(req, 5)
			if err := h.ai.StreamChat(ctx, req.Text, history, func(t string) { reply.WriteString(t); sse.token(t) }); err != nil {
				sse.token("抱歉，服务暂时不可用，请稍后再试。")
			}
			sse.done()
			h.saveMessages(name, req.SessionID, req.Text, reply.String(), "")
		}
	}
}

func (h *ChatHandler) streamReportCapture(ctx context.Context, sse *sseWriter, uid int, req model.ChatRequest) (string, string) {
	history := buildHistory(req, 5)

	// 让 LLM 从对话历史中提取完整工作内容
	extracted, err := h.ai.ExtractWorkContent(ctx, req.Text, history)
	if err != nil {
		logger.Warn("extract fallback, use raw text", "err", err)
		extracted = req.Text
	}

	// 带历史上下文验证是否为有效工作内容
	valid, reply, err := h.ai.ValidateWorkContent(ctx, extracted)
	if err != nil {
		logger.Warn("validate fallback", "err", err)
	}
	if !valid {
		sse.token(reply)
		sse.done()
		return reply, ""
	}

	// 充分性检查（对提取后的内容判断）
	sufficient, followUp, err := h.ai.AssessCompleteness(ctx, extracted)
	if err != nil {
		logger.Warn("assess fallback", "err", err)
		sufficient = true
	}
	logger.Info("chat.report.assess", "extracted", extracted, "sufficient", sufficient)
	if !sufficient && followUp != "" {
		sse.token(followUp)
		sse.done()
		return followUp, ""
	}

	summary, err := h.ai.StreamSummarize(ctx, extracted, sse.token)
	if err != nil {
		logger.Error("stream summarize failed", "err", err)
		sse.token("抱歉，摘要生成失败，请稍后重试。")
		sse.done()
		return "抱歉，摘要生成失败，请稍后重试。", ""
	}

	risks, _ := h.ai.DetectRisks(ctx, summary)
	logger.Info("chat.report.done", "uid", uid, "summary", summary, "risks", risks)

	h.pending.Store(uid, &model.PendingReport{
		Content: extracted, Summary: summary, Risks: risks,
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

	cfgJSON, _ := json.Marshal(meta)
	return summary, string(cfgJSON)
}

// buildHistory 从请求中提取最近 N 轮对话历史（用于 LLM 上下文）
func buildHistory(req model.ChatRequest, maxPairs int) []map[string]string {
	h := req.History
	// 最多取最近 maxPairs 对（user+assistant）
	if max := maxPairs * 2; len(h) > max {
		h = h[len(h)-max:]
	}
	msgs := make([]map[string]string, 0, len(h))
	for _, m := range h {
		if m.Role == "user" || m.Role == "assistant" {
			msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
		}
	}
	return msgs
}

// sessionIDStr 把 int64 session ID 转为 Data Asking 用的 string
func sessionIDStr(id *int64) string {
	if id == nil {
		return ""
	}
	return fmt.Sprintf("%d", *id)
}

// injectUserIdentity 把"我"替换为用户真名，让 Data Asking 知道身份
func injectUserIdentity(question, userName string) string {
	if strings.Contains(question, "我") {
		return strings.ReplaceAll(question, "我", userName) + fmt.Sprintf("（注：提问者是%s）", userName)
	}
	return question
}

func (h *ChatHandler) streamQueryCapture(ctx context.Context, sse *sseWriter, question string, req model.ChatRequest) (string, string) {
	history := buildHistory(req, 5)
	intent, _ := h.ai.ClassifyIntent(ctx, question, history)
	logger.Info("chat.query.intent", "question", question, "intent", intent)

	if intent == "chat" {
		var reply strings.Builder
		h.ai.StreamChat(ctx, question, history, func(t string) { reply.WriteString(t); sse.token(t) })
		sse.done()
		return reply.String(), ""
	}

	var answer strings.Builder
	var steps []string
	// Data Asking 用独立 session，不共用聊天 session（避免 agent 内部消息污染聊天历史）
	if err := h.ai.StreamQueryAnswer(ctx, question, "", func(t string) {
		answer.WriteString(t)
		sse.token(t)
	}, func(t string) {
		steps = append(steps, t)
		logger.Info("chat.query.thinking", "question", question, "step", len(steps), "content", t)
		sse.event("thinking", map[string]string{"text": t})
	}); err != nil {
		logger.Error("data asking failed", "err", err)
		if answer.Len() == 0 {
			msg := "数据查询服务暂时不可用，请稍后再试。"
			answer.WriteString(msg)
			sse.token(msg)
		}
	}
	logger.Info("chat.query.done", "question", question, "steps", len(steps), "answer", answer.String())
	if answer.Len() == 0 && len(steps) > 0 {
		// 用思考过程最后几步作为上下文，让 LLM 生成友好回复
		context := steps[len(steps)-1]
		if len(steps) >= 2 {
			context = steps[len(steps)-2] + "\n" + steps[len(steps)-1]
		}
		if err := h.ai.StreamEmptyQueryFallback(ctx, question, context, func(t string) {
			answer.WriteString(t)
			sse.token(t)
		}); err != nil {
			answer.WriteString("未查询到相关数据，请换个方式提问试试。")
			sse.token("未查询到相关数据，请换个方式提问试试。")
		}
	} else if answer.Len() == 0 {
		fallback := "未查询到相关数据，请换个方式提问试试。"
		answer.WriteString(fallback)
		sse.token(fallback)
	}
	sse.done()

	cfgJSON := ""
	if len(steps) > 0 {
		cfg, _ := json.Marshal(map[string]interface{}{"thinkingSteps": steps})
		cfgJSON = string(cfg)
	}
	return answer.String(), cfgJSON
}

func (h *ChatHandler) streamSummary(ctx context.Context, sse *sseWriter, uid int, name string, text string) {
	today := time.Now().Format("2006-01-02")
	weekday := [...]string{"日", "一", "二", "三", "四", "五", "六"}[time.Now().Weekday()]

	// 有用户输入则提取日期范围，否则默认最近7天
	var start, end string
	if strings.TrimSpace(text) != "" {
		dr, err := h.ai.ExtractDateRange(ctx, text, today, "星期"+weekday)
		if err != nil {
			logger.Warn("extract date range fallback", "err", err)
		} else {
			start, end = dr.Start, dr.End
		}
	}

	data, err := h.daily.GetMemberDateRangeData(ctx, uid, start, end)
	if err != nil {
		logger.Error("get data failed", "err", err)
	}
	if strings.TrimSpace(data) == "" {
		sse.token("该时间段暂无日报记录，无法生成周报。请先提交日报后再试。")
		sse.done()
		return
	}

	logger.Info("chat.summary", "uid", uid, "start", start, "end", end, "dataLen", len(data))
	md, err := h.ai.StreamWeeklySummary(ctx, name, data, sse.token)
	if err != nil {
		logger.Error("stream summary failed", "err", err)
		sse.token("抱歉，周报生成失败，请稍后重试。")
		sse.done()
		return
	}

	filename := fmt.Sprintf("周报_%s_%s.md", name, time.Now().Format("20060102"))
	dir := filepath.Join(".", "exports")
	os.MkdirAll(dir, 0755)
	fpath := filepath.Join(dir, filename)
	os.WriteFile(fpath, []byte(md), 0644)
	// 5 分钟后自动清理未下载的文件
	time.AfterFunc(5*time.Minute, func() { os.Remove(fpath) })

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
	defer os.Remove(path)
}
