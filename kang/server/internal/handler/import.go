package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"smart-daily/internal/logger"
	"smart-daily/internal/model"
	"smart-daily/internal/service"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ImportHandler struct {
	db          *gorm.DB
	ai          *service.AIService
	catalogSync *service.CatalogSync
	cache       sync.Map // token -> *previewCache
}

type previewCache struct {
	entries   []extractedEntry
	members   []model.Member
	createdAt time.Time
}

func NewImportHandler(db *gorm.DB, ai *service.AIService, cs *service.CatalogSync) *ImportHandler {
	h := &ImportHandler{db: db, ai: ai, catalogSync: cs}
	// Cleanup expired cache entries every 5 minutes
	go func() {
		for range time.Tick(5 * time.Minute) {
			h.cache.Range(func(k, v any) bool {
				if time.Since(v.(*previewCache).createdAt) > 10*time.Minute {
					h.cache.Delete(k)
				}
				return true
			})
		}
	}()
	return h
}

type docxSection struct {
	Date string `json:"date"`
	Text string `json:"text"`
}

type extractedEntry struct {
	Date    string `json:"date"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// Preview handles POST /api/import/preview — parse + LLM extract, return preview for confirmation
func (h *ImportHandler) Preview(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	logger.Info("import preview: start", "file", file.Filename, "size", file.Size)

	// Save to temp file
	tmp := filepath.Join(os.TempDir(), "import_"+file.Filename)
	if err := c.SaveUploadedFile(file, tmp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	defer os.Remove(tmp)

	// Parse docx
	parserPath := filepath.Join("server", "cmd", "docx_parser", "main.py")
	if _, err := os.Stat(parserPath); err != nil {
		parserPath = filepath.Join("cmd", "docx_parser", "main.py")
	}
	out, err := exec.CommandContext(c.Request.Context(), "python3", parserPath, tmp).Output()
	if err != nil {
		logger.Error("docx parse failed", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文档解析失败"})
		return
	}
	var sections []docxSection
	if err := json.Unmarshal(out, &sections); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析结果格式错误"})
		return
	}
	logger.Info("import preview: parsed", "sections", len(sections))

	if len(sections) == 0 {
		c.JSON(http.StatusOK, gin.H{"token": "", "entries": []extractedEntry{}, "unmatched_members": []string{}})
		return
	}

	// Parallel LLM extraction
	ctx := c.Request.Context()
	allEntries := h.extractAll(ctx, sections)

	// Match members
	var members []model.Member
	h.db.Find(&members)

	unmatchedSet := map[string]bool{}
	for _, e := range allEntries {
		if strings.TrimSpace(e.Content) != "" && matchMember(e.Name, members) == 0 {
			unmatchedSet[e.Name] = true
		}
	}
	var unmatched []string
	for name := range unmatchedSet {
		unmatched = append(unmatched, name)
	}

	// Cache for confirm step
	token := genToken()
	h.cache.Store(token, &previewCache{entries: allEntries, members: members, createdAt: time.Now()})

	logger.Info("import preview: done", "token", token, "entries", len(allEntries), "unmatched", len(unmatched))
	c.JSON(http.StatusOK, gin.H{
		"token":              token,
		"entries":            allEntries,
		"unmatched_members":  unmatched,
	})
}

// Confirm handles POST /api/import/confirm — save to DB + sync Catalog
func (h *ImportHandler) Confirm(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 token"})
		return
	}

	val, ok := h.cache.LoadAndDelete(req.Token)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "预览已过期，请重新上传"})
		return
	}
	cached := val.(*previewCache)
	logger.Info("import confirm: start", "token", req.Token, "entries", len(cached.entries))

	ctx := c.Request.Context()
	var skippedMembers []string
	skippedSet := map[string]bool{}

	// Prepare entries: match members, skip empty/unmatched
	type validEntry struct {
		memberID int
		date     string
		content  string
	}
	var valid []validEntry
	skipped := 0
	for _, e := range cached.entries {
		if strings.TrimSpace(e.Content) == "" {
			skipped++
			continue
		}
		memberID := matchMember(e.Name, cached.members)
		if memberID == 0 {
			if !skippedSet[e.Name] {
				skippedSet[e.Name] = true
				skippedMembers = append(skippedMembers, e.Name)
			}
			skipped++
			continue
		}
		valid = append(valid, validEntry{memberID, e.Date, e.Content})
	}

	// Count existing for merged/imported stats
	type key struct{ mid int; date string }
	existingKeys := map[key]bool{}
	if len(valid) > 0 {
		var existing []model.DailyEntry
		h.db.WithContext(ctx).Where("source = 'import'").Select("member_id, daily_date").Find(&existing)
		for _, e := range existing {
			existingKeys[key{e.MemberID, e.DailyDate}] = true
		}
	}

	// Bulk delete + insert
	var savedEntries []model.DailyEntry
	merged, imported := 0, 0
	if len(valid) > 0 {
		// Delete existing entries that will be replaced
		var delKeys [][]interface{}
		for _, v := range valid {
			delKeys = append(delKeys, []interface{}{v.memberID, v.date})
		}
		h.db.WithContext(ctx).Where("source = 'import' AND (member_id, daily_date) IN ?", delKeys).Delete(&model.DailyEntry{})

		// Bulk insert
		now := time.Now()
		for _, v := range valid {
			entry := model.DailyEntry{
				MemberID: v.memberID, DailyDate: v.date,
				Content: v.content, Summary: v.content, Source: "import",
			}
			entry.CreatedAt = now
			savedEntries = append(savedEntries, entry)
			if existingKeys[key{v.memberID, v.date}] {
				merged++
			} else {
				imported++
			}
		}
		h.db.WithContext(ctx).Create(&savedEntries)
	}

	if len(savedEntries) > 0 && h.catalogSync != nil && h.catalogSync.Ready() {
		h.catalogSync.SyncDailyEntries(ctx, savedEntries)
		logger.Info("import catalog sync done", "entries", len(savedEntries))
	}

	logger.Info("import confirm: done", "imported", imported, "merged", merged, "skipped", skipped)
	c.JSON(http.StatusOK, gin.H{
		"imported":        imported,
		"merged":          merged,
		"skipped":         skipped,
		"skipped_members": skippedMembers,
		"total":           len(cached.entries),
	})
}

// --- internal helpers ---

func (h *ImportHandler) extractAll(ctx context.Context, sections []docxSection) []extractedEntry {
	batchSize := 1
	type batchResult struct {
		entries []extractedEntry
		err     error
	}
	var batches [][]docxSection
	for i := 0; i < len(sections); i += batchSize {
		end := i + batchSize
		if end > len(sections) {
			end = len(sections)
		}
		batches = append(batches, sections[i:end])
	}

	maxConcurrent := 50
	sem := make(chan struct{}, maxConcurrent)
	results := make([]batchResult, len(batches))
	var wg sync.WaitGroup
	for idx, batch := range batches {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, b []docxSection) {
			defer wg.Done()
			defer func() { <-sem }()
			logger.Info("import: extracting batch", "batch", i, "sections", len(b))
			entries, err := h.extractBatch(ctx, b)
			logger.Info("import: batch extracted", "batch", i, "entries", len(entries), "err", err)
			results[i] = batchResult{entries, err}
		}(idx, batch)
	}
	wg.Wait()

	var all []extractedEntry
	for _, br := range results {
		if br.err == nil {
			all = append(all, br.entries...)
		}
	}
	return all
}

var dateRe = regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)

func (h *ImportHandler) extractBatch(ctx context.Context, sections []docxSection) ([]extractedEntry, error) {
	var sb strings.Builder
	for _, s := range sections {
		sb.WriteString("--- " + s.Date + " ---\n")
		sb.WriteString(s.Text + "\n\n")
	}

	system := `你是日报数据提取助手。从以下日报文档中提取每个人每天的工作内容。
文档格式可能各不相同（列名、列数、排列方式都可能不同），你需要自行理解文档结构。

核心任务：提取出"谁、哪天、做了什么"三要素。

规则：
- 智能识别文档结构，不要假设固定的列名或格式
- 综合所有列的信息生成完整的工作描述。例如某列是项目/模块名，另一列是具体内容，应合并为"项目名: 具体内容"或自然语句
- 完整保留原文内容，不要缩写、省略或截断任何文字
- 某人某天所有列都为空则跳过
- 日期格式统一转为 YYYY-MM-DD
- content 字段用完整的文本描述，多项工作用逗号分隔
- 只输出 JSON 数组，不加任何解释

输出格式：[{"date":"2026-02-13","name":"蒯伟康","content":"智能daily: 跑通moi-dev环境, 页面初步调通, 已部署"}]`

	result, err := h.ai.DoChat(ctx, system, sb.String())
	if err != nil {
		return nil, err
	}

	result = strings.TrimSpace(result)
	if i := strings.Index(result, "["); i >= 0 {
		if j := strings.LastIndex(result, "]"); j > i {
			result = result[i : j+1]
		}
	}

	var entries []extractedEntry
	if err := json.Unmarshal([]byte(result), &entries); err != nil {
		return nil, fmt.Errorf("parse LLM result: %w (raw: %.200s)", err, result)
	}
	return entries, nil
}

func matchMember(name string, members []model.Member) int {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0
	}
	for _, m := range members {
		if m.Name == name {
			return m.ID
		}
	}
	for _, m := range members {
		if strings.Contains(m.Name, name) || strings.Contains(name, m.Name) {
			return m.ID
		}
	}
	return 0
}

func genToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
