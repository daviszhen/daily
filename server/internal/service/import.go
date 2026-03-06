package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"smart-daily/internal/logger"
	"smart-daily/internal/model"
	"smart-daily/internal/repository"
	"strings"
	"sync"
	"time"
)

type ImportService struct {
	ai          *AIService
	memberRepo  *repository.MemberRepo
	dailyRepo   *repository.DailyRepo
	topicRepo   *repository.TopicRepo
	catalogSync *CatalogSync
}

func NewImportService(ai *AIService, mr *repository.MemberRepo, dr *repository.DailyRepo, tr *repository.TopicRepo, cs *CatalogSync) *ImportService {
	return &ImportService{ai: ai, memberRepo: mr, dailyRepo: dr, topicRepo: tr, catalogSync: cs}
}

type ExtractedEntry struct {
	Date    string `json:"date"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type MemberDecision struct {
	Action   string `json:"action"`
	Name     string `json:"name"`
	MemberID int    `json:"member_id"`
	TeamID   int    `json:"team_id"`
	Role     string `json:"role"`
}

type PreviewResult struct {
	Entries   []ExtractedEntry
	Members   []model.Member
	Unmatched []string
}

type ConfirmResult struct {
	Imported int `json:"imported"`
	Merged   int `json:"merged"`
	Skipped  int `json:"skipped"`
	Total    int `json:"total"`
}

type DocxSection struct {
	Date string `json:"date"`
	Text string `json:"text"`
}

// Extract parses sections and returns preview data.
func (s *ImportService) Extract(ctx context.Context, sections []DocxSection) (*PreviewResult, error) {
	if len(sections) == 0 {
		return &PreviewResult{Entries: []ExtractedEntry{}, Unmatched: []string{}}, nil
	}

	members, _ := s.memberRepo.ListActive(ctx)
	var knownNames []string
	for _, m := range members {
		knownNames = append(knownNames, m.Name)
	}

	var entries []ExtractedEntry
	if entries = extractProgrammatic(sections); len(entries) > 0 {
		logger.Info("import: programmatic extraction", "entries", len(entries))
	} else {
		logger.Info("import: falling back to LLM extraction")
		entries = s.extractAll(ctx, sections, knownNames)
	}

	unmatchedSet := map[string]bool{}
	for _, e := range entries {
		if strings.TrimSpace(e.Content) != "" && repository.MatchByName(e.Name, members) == 0 {
			unmatchedSet[e.Name] = true
		}
	}
	unmatched := make([]string, 0, len(unmatchedSet))
	for name := range unmatchedSet {
		unmatched = append(unmatched, name)
	}

	return &PreviewResult{Entries: entries, Members: members, Unmatched: unmatched}, nil
}

// Confirm processes member decisions and saves entries to DB.
func (s *ImportService) Confirm(ctx context.Context, entries []ExtractedEntry, members []model.Member, decisions map[string]MemberDecision) (*ConfirmResult, error) {
	ignoredNames := map[string]bool{}
	nameToMemberID := map[string]int{}

	for name, d := range decisions {
		switch d.Action {
		case "ignore":
			ignoredNames[name] = true
		case "map":
			if d.MemberID > 0 {
				nameToMemberID[name] = d.MemberID
			}
		case "create":
			createName := strings.TrimSpace(d.Name)
			if createName == "" {
				createName = name
			}
			if existingID := repository.MatchByName(createName, members); existingID != 0 {
				if createName != name || repository.MatchByName(name, members) == 0 {
					nameToMemberID[name] = existingID
				}
				continue
			}
			newMember := model.Member{
				Username: "user_" + randHex(8),
				Password: "$2a$10$sH3qZ9F0SIrCWpcOi9oWDO6EjbWMRs4X/8d35hphzkYRRM.ESRsa.",
				Name:     createName,
				Role:     d.Role, TeamID: d.TeamID, Status: "active",
			}
			if newMember.Role == "" {
				newMember.Role = "开发工程师"
			}
			if err := s.memberRepo.Create(ctx, &newMember); err != nil {
				logger.Warn("import: create member failed", "name", createName, "err", err)
				continue
			}
			members = append(members, newMember)
			if createName != name {
				nameToMemberID[name] = newMember.ID
			}
		}
	}

	// Auto-create for unmatched names without decisions
	for _, e := range entries {
		name := strings.TrimSpace(e.Name)
		if name == "" || strings.TrimSpace(e.Content) == "" {
			continue
		}
		if ignoredNames[name] || nameToMemberID[name] > 0 || repository.MatchByName(name, members) != 0 {
			continue
		}
		newMember := model.Member{
			Username: "user_" + randHex(8),
			Password: "$2a$10$sH3qZ9F0SIrCWpcOi9oWDO6EjbWMRs4X/8d35hphzkYRRM.ESRsa.",
			Name:     name, Role: "开发工程师", Status: "active",
		}
		if err := s.memberRepo.Create(ctx, &newMember); err != nil {
			continue
		}
		members = append(members, newMember)
	}

	// Build valid entries
	type validEntry struct {
		memberID int
		date, content string
	}
	var valid []validEntry
	skipped := 0
	for _, e := range entries {
		if strings.TrimSpace(e.Content) == "" {
			skipped++
			continue
		}
		name := strings.TrimSpace(e.Name)
		if ignoredNames[name] {
			skipped++
			continue
		}
		memberID := nameToMemberID[name]
		if memberID == 0 {
			memberID = repository.MatchByName(name, members)
		}
		if memberID == 0 {
			skipped++
			continue
		}
		valid = append(valid, validEntry{memberID, e.Date, e.Content})
	}

	// Find existing imports for merged/imported stats
	type key struct{ mid int; date string }
	existingKeys := map[key]bool{}
	if len(valid) > 0 {
		existing, _ := s.dailyRepo.FindExistingImportKeys(ctx)
		for k := range existing {
			if mid, ok := k[0].(int); ok {
				if date, ok := k[1].(string); ok {
					existingKeys[key{mid, date}] = true
				}
			}
		}
	}

	// Bulk save
	var savedEntries []model.DailyEntry
	merged, imported := 0, 0
	if len(valid) > 0 {
		var delKeys [][]interface{}
		now := time.Now()
		for _, v := range valid {
			delKeys = append(delKeys, []interface{}{v.memberID, v.date})
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
		s.dailyRepo.BulkReplaceImportEntries(ctx, delKeys, savedEntries)

		// Bulk replace summaries (was 2645 individual UpsertSummary calls)
		var summaries []model.DailySummary
		for _, v := range valid {
			summaries = append(summaries, model.DailySummary{
				MemberID: v.memberID, DailyDate: v.date, Summary: v.content,
			})
		}
		s.dailyRepo.BulkReplaceSummaries(ctx, delKeys, summaries)
	}

	// Catalog sync — use background context so frontend disconnect won't cancel it
	bgCtx := context.Background()
	if len(savedEntries) > 0 && s.catalogSync != nil && s.catalogSync.Ready() {
		s.catalogSync.SyncDailyEntries(bgCtx, savedEntries)
	}

	// Extract topics async
	if len(savedEntries) > 0 {
		go s.batchExtractTopics(savedEntries, members)
	}

	return &ConfirmResult{Imported: imported, Merged: merged, Skipped: skipped, Total: len(entries)}, nil
}

// --- extraction helpers ---

var dateRe = regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)

func extractProgrammatic(sections []DocxSection) []ExtractedEntry {
	skipFirst := map[string]bool{"成员": true, "迭代": true}
	var all []ExtractedEntry
	for _, s := range sections {
		dm := dateRe.FindStringSubmatch(s.Date)
		if dm == nil {
			continue
		}
		date := fmt.Sprintf("%s-%02s-%02s",
			dm[1], fmt.Sprintf("%02d", mustAtoi(dm[2])), fmt.Sprintf("%02d", mustAtoi(dm[3])))

		lines := strings.Split(s.Text, "\n")
		if len(lines) < 2 || !strings.Contains(lines[0], "\t") {
			return nil
		}
		startIdx := -1
		for i, line := range lines {
			if strings.HasPrefix(line, "成员\t") {
				startIdx = i + 1
				break
			}
		}
		if startIdx < 0 {
			startIdx = 1
		}
		for _, line := range lines[startIdx:] {
			cols := strings.Split(line, "\t")
			if len(cols) < 2 {
				continue
			}
			name := strings.ReplaceAll(strings.TrimSpace(cols[0]), " ", "")
			if name == "" || skipFirst[name] {
				continue
			}
			var parts []string
			for _, c := range cols[1:] {
				if c = strings.TrimSpace(c); c != "" {
					parts = append(parts, c)
				}
			}
			if len(parts) == 0 {
				continue
			}
			all = append(all, ExtractedEntry{Date: date, Name: name, Content: strings.Join(parts, "; ")})
		}
	}
	return all
}

func (s *ImportService) extractAll(ctx context.Context, sections []DocxSection, knownNames []string) []ExtractedEntry {
	sem := make(chan struct{}, 50)
	results := make([][]ExtractedEntry, len(sections))
	var wg sync.WaitGroup
	for i, sec := range sections {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, sec DocxSection) {
			defer wg.Done()
			defer func() { <-sem }()
			entries, err := s.extractBatch(ctx, sec, knownNames)
			if err == nil {
				results[idx] = entries
			}
		}(i, sec)
	}
	wg.Wait()

	var all []ExtractedEntry
	for _, r := range results {
		all = append(all, r...)
	}
	return all
}

func (s *ImportService) extractBatch(ctx context.Context, sec DocxSection, knownNames []string) ([]ExtractedEntry, error) {
	var extra strings.Builder
	if len(knownNames) > 0 {
		extra.WriteString("\n已知成员列表：" + strings.Join(knownNames, "、") + "\n请优先匹配这些名字。name 字段只填真实人名（通常2-4个中文字），不要把项目名、模块名、产品名当作人名。\n")
	}

	system := `你是日报数据提取助手。从以下日报文档中提取每个人每天的工作内容。
` + extra.String() + `
核心任务：提取出"谁、哪天、做了什么"三要素。

规则：
- name 字段只填人名，不要填项目名、产品名、模块名
- 综合所有列的信息生成完整的工作描述。例如某列是项目/模块名，另一列是具体内容，应合并为"项目名: 具体内容"或自然语句
- 完整保留原文内容，不要缩写、省略或截断任何文字
- 某人某天所有列都为空则跳过
- 日期格式统一转为 YYYY-MM-DD
- content 字段用完整的文本描述，多项工作用逗号分隔
- 只输出 JSON 数组，不加任何解释

输出格式：[{"date":"2026-02-13","name":"蒯伟康","content":"智能daily: 跑通moi-dev环境, 页面初步调通, 已部署"}]`

	input := "--- " + sec.Date + " ---\n" + sec.Text
	result, err := s.ai.DoChat(ctx, system, input)
	if err != nil {
		return nil, err
	}

	result = strings.TrimSpace(result)
	if i := strings.Index(result, "["); i >= 0 {
		if j := strings.LastIndex(result, "]"); j > i {
			result = result[i : j+1]
		}
	}
	var entries []ExtractedEntry
	if err := json.Unmarshal([]byte(result), &entries); err != nil {
		return nil, fmt.Errorf("parse LLM result: %w (raw: %.200s)", err, result)
	}
	for i := range entries {
		entries[i].Name = strings.ReplaceAll(strings.TrimSpace(entries[i].Name), " ", "")
	}
	return entries, nil
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

func (s *ImportService) batchExtractTopics(entries []model.DailyEntry, members []model.Member) {
	ctx := context.Background()

	// Delete old topic_activities for these entries (idempotent re-import)
	var entryIDs []int
	for _, e := range entries {
		entryIDs = append(entryIDs, e.ID)
	}
	if len(entryIDs) > 0 {
		s.topicRepo.DeleteByEntryIDs(ctx, entryIDs)
	}

	existingTopics, _ := s.topicRepo.ListDistinctTopics(ctx)

	nameMap := make(map[int]string)
	for _, m := range members {
		nameMap[m.ID] = m.Name
	}

	// Group entries into batches of 20
	const batchSize = 20
	type batch struct {
		contents map[int]string
		entries  map[int]model.DailyEntry
	}
	var batches []batch
	cur := batch{contents: map[int]string{}, entries: map[int]model.DailyEntry{}}
	for _, e := range entries {
		cur.contents[e.ID] = e.Content
		cur.entries[e.ID] = e
		if len(cur.contents) >= batchSize {
			batches = append(batches, cur)
			cur = batch{contents: map[int]string{}, entries: map[int]model.DailyEntry{}}
		}
	}
	if len(cur.contents) > 0 {
		batches = append(batches, cur)
	}

	sem := make(chan struct{}, 10)
	var mu sync.Mutex
	var allItems []model.TopicActivity
	var wg sync.WaitGroup

	for _, b := range batches {
		wg.Add(1)
		sem <- struct{}{}
		go func(b batch) {
			defer wg.Done()
			defer func() { <-sem }()
			result, err := s.ai.ExtractTopicsBatch(ctx, b.contents, existingTopics)
			if err != nil {
				return
			}
			mu.Lock()
			for entryID, topics := range result {
				e, ok := b.entries[entryID]
				if !ok {
					continue
				}
				date := e.DailyDate
				if len(date) > 10 {
					date = date[:10]
				}
				for _, t := range topics {
					allItems = append(allItems, model.TopicActivity{
						Topic: t, MemberID: e.MemberID, MemberName: nameMap[e.MemberID],
						DailyDate: date, Content: e.Content, EntryID: e.ID,
					})
				}
			}
			mu.Unlock()
		}(b)
	}
	wg.Wait()

	if len(allItems) > 0 {
		topicSet := map[string]bool{}
		for _, item := range allItems {
			topicSet[item.Topic] = true
		}
		var topicNames []string
		for t := range topicSet {
			topicNames = append(topicNames, t)
		}
		s.topicRepo.EnsureTopics(ctx, topicNames)
		s.topicRepo.BatchCreate(ctx, allItems)
		logger.Info("import: topics extracted", "entries", len(entries), "activities", len(allItems))
	}
}
