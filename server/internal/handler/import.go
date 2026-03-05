package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"smart-daily/internal/logger"
	"smart-daily/internal/middleware"
	"smart-daily/internal/model"
	"smart-daily/internal/service"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ImportHandler struct {
	importSvc *service.ImportService
	cache     sync.Map // token -> *previewCache
}

type previewCache struct {
	entries   []service.ExtractedEntry
	members   []model.Member
	createdAt time.Time
}

func NewImportHandler(importSvc *service.ImportService) *ImportHandler {
	h := &ImportHandler{importSvc: importSvc}
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

// Preview handles POST /api/import/preview
func (h *ImportHandler) Preview(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}
	logger.Info("import preview: start", "file", file.Filename, "size", file.Size)

	tmp := filepath.Join(os.TempDir(), "import_"+file.Filename)
	if err := c.SaveUploadedFile(file, tmp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	defer os.Remove(tmp)

	// Parse docx via Python
	parserPath := filepath.Join("server", "cmd", "docx_parser", "main.py")
	if _, err := os.Stat(parserPath); err != nil {
		parserPath = filepath.Join("cmd", "docx_parser", "main.py")
	}
	out, err := exec.CommandContext(c.Request.Context(), "python3", parserPath, tmp).Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		logger.Error("docx parse failed", "err", err, "stderr", stderr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文档解析失败"})
		return
	}
	var sections []service.DocxSection
	if err := json.Unmarshal(out, &sections); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析结果格式错误"})
		return
	}
	logger.Info("import preview: parsed", "sections", len(sections))

	result, err := h.importSvc.Extract(c.Request.Context(), sections)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token := genToken()
	h.cache.Store(token, &previewCache{entries: result.Entries, members: result.Members, createdAt: time.Now()})

	// Non-admin: filter to only current user's entries
	entries := result.Entries
	if !middleware.IsAdmin(c) {
		userName := c.GetString("user_name")
		var filtered []service.ExtractedEntry
		for _, e := range entries {
			if e.Name == userName {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	type memberInfo struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var memberList []memberInfo
	for _, m := range result.Members {
		memberList = append(memberList, memberInfo{m.ID, m.Name})
	}

	logger.Info("import preview: done", "token", token, "entries", len(entries), "unmatched", len(result.Unmatched))
	c.JSON(http.StatusOK, gin.H{
		"token":             token,
		"entries":           entries,
		"unmatched_members": result.Unmatched,
		"members":           memberList,
	})
}

// Confirm handles POST /api/import/confirm
func (h *ImportHandler) Confirm(c *gin.Context) {
	var req struct {
		Token           string                            `json:"token"`
		MemberDecisions map[string]service.MemberDecision `json:"member_decisions"`
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

	// Non-admin: filter to only current user's entries
	entries := cached.entries
	if !middleware.IsAdmin(c) {
		userName := c.GetString("user_name")
		var filtered []service.ExtractedEntry
		for _, e := range entries {
			if e.Name == userName {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	logger.Info("import confirm: start", "token", req.Token, "entries", len(entries), "decisions", len(req.MemberDecisions))

	result, err := h.importSvc.Confirm(c.Request.Context(), entries, cached.members, req.MemberDecisions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Info("import confirm: done", "imported", result.Imported, "merged", result.Merged, "skipped", result.Skipped)
	c.JSON(http.StatusOK, result)
}

func genToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
