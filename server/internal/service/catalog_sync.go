package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"smart-daily/internal/logger"
	"smart-daily/internal/model"
	"strconv"
	"strings"
	"time"

	sdk "github.com/matrixorigin/moi-go-sdk"
	"gorm.io/gorm"
)

type CatalogSync struct {
	raw        *sdk.RawClient
	sdk        *sdk.SDKClient
	databaseID sdk.DatabaseID
	tableIDs   map[string]sdk.TableID // table name -> table ID
	ready      bool
	baseURL    string
	apiKey     string
}

func NewCatalogSync(raw *sdk.RawClient, catalogID int64, dbName, baseURL, apiKey string) *CatalogSync {
	s := &CatalogSync{
		raw:      raw,
		sdk:      sdk.NewSDKClient(raw),
		tableIDs: make(map[string]sdk.TableID),
		baseURL:  baseURL,
		apiKey:   apiKey,
	}
	s.discover(catalogID, dbName)
	return s
}

func (s *CatalogSync) discover(catalogID int64, dbName string) {
	ctx := context.Background()

	// 1. find database by name
	dbList, err := s.raw.ListDatabases(ctx, &sdk.DatabaseListRequest{CatalogID: sdk.CatalogID(catalogID)})
	if err != nil {
		logger.Warn("catalog: list databases failed", "err", err)
		return
	}
	for _, db := range dbList.List {
		if db.DatabaseName == dbName {
			s.databaseID = db.DatabaseID
			break
		}
	}
	if s.databaseID == 0 {
		logger.Warn("catalog: database not found in catalog, Data Asking will not work", "name", dbName, "catalog_id", catalogID)
		return
	}

	// 2. find existing tables
	children, err := s.raw.GetDatabaseChildren(ctx, &sdk.DatabaseChildrenRequest{DatabaseID: s.databaseID})
	if err != nil {
		logger.Warn("catalog: list tables failed", "err", err)
		return
	}
	existing := map[string]bool{}
	for _, child := range children.List {
		if child.Typ == "table" {
			id, _ := strconv.ParseInt(child.ID, 10, 64)
			s.tableIDs[child.Name] = sdk.TableID(id)
			existing[child.Name] = true
		}
	}

	// 3. record existing table IDs (schema sync happens later via SyncSchemaFromDB)
	s.ready = true
	logger.Info("catalog: discovered", "database_id", s.databaseID, "tables", s.tableIDs)
}

// syncTables lists tables to sync to Catalog. Order matters for display.
var syncTables = []string{"members", "teams", "daily_entries", "daily_summaries", "topics", "topic_activities"}

// columnComments provides semantic descriptions for Catalog and NL2SQL Knowledge.
// This is the SINGLE source of truth — add new tables/columns here, everything auto-syncs.
var columnComments = map[string]map[string]string{
	"members": {
		"id": "主键", "username": "登录用户名", "password": "密码哈希",
		"name": "中文姓名", "avatar": "头像URL", "role": "职位角色",
		"team": "所属团队名称", "team_id": "所属团队ID,关联teams.id",
		"status": "状态:active/deleted", "is_admin": "是否管理员",
	},
	"teams": {
		"id": "主键", "name": "团队名称",
	},
	"daily_entries": {
		"id": "主键", "member_id": "关联members.id", "daily_date": "日报日期",
		"content": "原始工作内容", "summary": "AI摘要",
		"source": "来源:chat/import", "created_at": "创建时间",
	},
	"daily_summaries": {
		"id": "主键", "member_id": "关联members.id", "daily_date": "日报日期",
		"summary": "当天合并总结", "status": "工作状态",
		"risk": "风险项", "blocker": "阻塞问题",
	},
	"topics": {
		"id": "主键", "name": "Topic名称", "description": "描述",
		"status": "active/resolved", "created_at": "创建时间", "resolved_at": "解决时间",
	},
	"topic_activities": {
		"id": "主键", "topic": "Topic名称", "member_id": "成员ID",
		"member_name": "成员姓名", "daily_date": "日期",
		"content": "工作内容", "entry_id": "关联daily_entries.id",
	},
}

// SyncSchemaFromDB reads actual DB columns and creates missing Catalog tables.
// Existing tables are left untouched (no delete, no rebuild).
func (s *CatalogSync) SyncSchemaFromDB(db *gorm.DB) {
	if !s.ready {
		return
	}
	ctx := context.Background()
	for _, table := range syncTables {
		if _, exists := s.tableIDs[table]; exists {
			continue // already in Catalog, skip
		}
		var cols []struct {
			Field string
			Type  string
			Key   string
		}
		if err := db.Raw("SHOW COLUMNS FROM " + table).Scan(&cols).Error; err != nil {
			logger.Warn("catalog: show columns failed", "table", table, "err", err)
			continue
		}
		comments := columnComments[table]
		var sdkCols []sdk.Column
		for _, c := range cols {
			sdkCols = append(sdkCols, sdk.Column{
				Name: c.Field, Type: mapDBType(c.Type), IsPk: c.Key == "PRI", Comment: comments[c.Field],
			})
		}
		resp, err := s.raw.CreateTable(ctx, &sdk.TableCreateRequest{
			DatabaseID: s.databaseID, Name: table, Columns: sdkCols, Comment: table,
		})
		if err != nil {
			if strings.Contains(err.Error(), "uplicate") {
				continue
			}
			logger.Warn("catalog: create table failed", "table", table, "err", err)
			continue
		}
		s.tableIDs[table] = resp.TableID
		logger.Info("catalog: table created", "table", table, "columns", len(sdkCols))
	}
}

// mapDBType converts MySQL column types to Catalog types.
func mapDBType(dbType string) string {
	t := strings.ToUpper(dbType)
	switch {
	case strings.HasPrefix(t, "INT"), strings.HasPrefix(t, "BIGINT"), strings.HasPrefix(t, "TINYINT"):
		return "INT"
	case strings.HasPrefix(t, "VARCHAR"):
		return dbType // keep VARCHAR(N)
	case strings.HasPrefix(t, "TEXT"), strings.HasPrefix(t, "LONGTEXT"), strings.HasPrefix(t, "MEDIUMTEXT"):
		return "TEXT"
	case strings.HasPrefix(t, "DATE") && !strings.HasPrefix(t, "DATETIME"):
		return "DATE"
	case strings.HasPrefix(t, "DATETIME"), strings.HasPrefix(t, "TIMESTAMP"):
		return "DATETIME"
	case strings.HasPrefix(t, "BOOL"):
		return "BOOL"
	default:
		return dbType
	}
}

// ColumnComments returns the comment map (used by SeedKnowledge to generate descriptions).
func ColumnComments() map[string]map[string]string { return columnComments }

// SyncTables returns the list of synced table names.
func SyncTableNames() []string { return syncTables }

// KnowledgeEntries returns all NL2SQL knowledge entries.
// Used by both catalog_init (first-time setup) and server startup (incremental sync).
func KnowledgeEntries() []sdk.NL2SQLKnowledgeCreateRequest {
	return []sdk.NL2SQLKnowledgeCreateRequest{
		// glossary
		{Type: "glossary", Key: "日报", Value: []string{"daily_entries表中的一条记录，代表某个团队成员某天提交的工作汇报"}},
		{Type: "glossary", Key: "成员", Value: []string{"members表中的记录，代表团队中的一个人"}},
		{Type: "glossary", Key: "风险", Value: []string{"daily_summaries.risk字段，AI从日报内容中检测到的潜在风险项"}},
		{Type: "glossary", Key: "摘要", Value: []string{"daily_summaries.summary字段，AI对日报原始内容的精炼总结"}},
		{Type: "glossary", Key: "Topic", Value: []string{"topics表中的记录，代表一个研发主题（如MOI/问数/内核），从日报中自动提取"}},
		{Type: "glossary", Key: "团队", Value: []string{"teams表中的记录，代表一个团队。通过members.team_id关联成员"}},
		// synonyms
		{Type: "synonyms", Key: "姓名/名字/谁/人员/同事", Value: []string{"团队成员的真实姓名"}, AssociateTables: []string{"members,name"}},
		{Type: "synonyms", Key: "日期/时间/哪天/什么时候", Value: []string{"日报的日期"}, AssociateTables: []string{"daily_entries,daily_date"}},
		{Type: "synonyms", Key: "工作内容/做了什么/干了啥", Value: []string{"日报原始内容"}, AssociateTables: []string{"daily_entries,content"}},
		{Type: "synonyms", Key: "角色/职位/岗位", Value: []string{"成员的职位角色"}, AssociateTables: []string{"members,role"}},
		{Type: "synonyms", Key: "主题/话题/方向", Value: []string{"研发主题"}, AssociateTables: []string{"topic_activities,topic"}},
		{Type: "synonyms", Key: "团队/组/部门", Value: []string{"所属团队"}, AssociateTables: []string{"teams,name"}},
		// logic
		{Type: "logic", Key: "查询某人的日报时，需要通过daily_entries.member_id关联members.id来获取姓名", Value: []string{"JOIN members ON daily_entries.member_id = members.id"}},
		{Type: "logic", Key: "今天指CURDATE()，本周指从本周一到今天，本月指从本月1号到今天", Value: []string{"日期范围计算规则"}},
		{Type: "logic", Key: "判断谁没交日报：用members LEFT JOIN daily_entries，找daily_entries.id IS NULL的成员", Value: []string{"缺勤查询逻辑"}},
		{Type: "logic", Key: "统计提交率：已提交人数/总人数*100，排除status='deleted'的成员", Value: []string{"提交率计算排除已删除成员"}},
		{Type: "logic", Key: "查询某个topic的参与情况：通过topic_activities表按topic字段聚合", Value: []string{"SELECT topic, member_name, daily_date, content FROM topic_activities WHERE topic = 'xxx'"}},
		{Type: "logic", Key: "查询团队成员：通过members.team_id关联teams.id", Value: []string{"JOIN teams ON members.team_id = teams.id"}},
		// case_library
		{Type: "case_library", Key: "今天谁没交日报", Value: []string{"SELECT m.name FROM members m LEFT JOIN daily_entries de ON m.id = de.member_id AND de.daily_date = CURDATE() WHERE de.id IS NULL AND m.status != 'deleted'"}},
		{Type: "case_library", Key: "彭振这周做了什么", Value: []string{"SELECT de.daily_date, de.summary FROM daily_entries de JOIN members m ON de.member_id = m.id WHERE m.name = '彭振' AND de.daily_date >= DATE_SUB(CURDATE(), INTERVAL WEEKDAY(CURDATE()) DAY)"}},
		{Type: "case_library", Key: "本周有哪些风险", Value: []string{"SELECT m.name, ds.daily_date, ds.risk FROM daily_summaries ds JOIN members m ON ds.member_id = m.id WHERE ds.risk != '' AND ds.daily_date >= DATE_SUB(CURDATE(), INTERVAL WEEKDAY(CURDATE()) DAY)"}},
		{Type: "case_library", Key: "最近一周的日报提交情况", Value: []string{"SELECT de.daily_date, COUNT(*) as submitted FROM daily_entries de WHERE de.daily_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY) GROUP BY de.daily_date ORDER BY de.daily_date"}},
		{Type: "case_library", Key: "哪些topic持续超过一周", Value: []string{"SELECT topic, MIN(daily_date) as start_date, MAX(daily_date) as end_date, DATEDIFF(MAX(daily_date), MIN(daily_date)) as days, COUNT(DISTINCT member_id) as people FROM topic_activities GROUP BY topic HAVING days > 7 ORDER BY days DESC"}},
		{Type: "case_library", Key: "某个团队有哪些人", Value: []string{"SELECT m.name, m.role FROM members m JOIN teams t ON m.team_id = t.id WHERE t.name = '某团队' AND m.status != 'deleted'"}},
	}
}

func (s *CatalogSync) Ready() bool     { return s.ready }
func (s *CatalogSync) DatabaseID() int { return int(s.databaseID) }

func (s *CatalogSync) SyncDailySummary(ctx context.Context, entryID, memberID int, date, content, summary, risk string) {
	if !s.ready {
		logger.Warn("catalog sync: skipped, not ready")
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")

	entryCsv := fmt.Sprintf("%d,%d,%s,%s,%s,chat,%s\n",
		entryID, memberID, dateOnly(date), esc(content), esc(summary), now)
	s.importCSV(ctx, s.tableIDs["daily_entries"], entryCsv, fmt.Sprintf("entry_%d.csv", entryID),
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "member_id", Column: "member_id", ColNumInFile: 2},
			{TableColumn: "daily_date", Column: "daily_date", ColNumInFile: 3},
			{TableColumn: "content", Column: "content", ColNumInFile: 4},
			{TableColumn: "summary", Column: "summary", ColNumInFile: 5},
			{TableColumn: "source", Column: "source", ColNumInFile: 6},
			{TableColumn: "created_at", Column: "created_at", ColNumInFile: 7},
		})

	sumCsv := fmt.Sprintf("%d,%d,%s,%s,,%s,\n",
		entryID, memberID, dateOnly(date), esc(summary), esc(risk))
	s.importCSV(ctx, s.tableIDs["daily_summaries"], sumCsv, fmt.Sprintf("sum_%d.csv", entryID),
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "member_id", Column: "member_id", ColNumInFile: 2},
			{TableColumn: "daily_date", Column: "daily_date", ColNumInFile: 3},
			{TableColumn: "summary", Column: "summary", ColNumInFile: 4},
			{TableColumn: "status", Column: "status", ColNumInFile: 5},
			{TableColumn: "risk", Column: "risk", ColNumInFile: 6},
			{TableColumn: "blocker", Column: "blocker", ColNumInFile: 7},
		})
}

func (s *CatalogSync) SyncDailyEntries(ctx context.Context, entries []model.DailyEntry) {
	if !s.ready || len(entries) == 0 {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	var entryBuf, sumBuf bytes.Buffer
	for _, e := range entries {
		fmt.Fprintf(&entryBuf, "%d,%d,%s,%s,%s,%s,%s\n",
			e.ID, e.MemberID, dateOnly(e.DailyDate), esc(e.Content), esc(e.Summary), e.Source, now)
		fmt.Fprintf(&sumBuf, "%d,%d,%s,%s,,,\n",
			e.ID, e.MemberID, dateOnly(e.DailyDate), esc(e.Summary))
	}
	s.importCSV(ctx, s.tableIDs["daily_entries"], entryBuf.String(), "import_entries.csv",
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "member_id", Column: "member_id", ColNumInFile: 2},
			{TableColumn: "daily_date", Column: "daily_date", ColNumInFile: 3},
			{TableColumn: "content", Column: "content", ColNumInFile: 4},
			{TableColumn: "summary", Column: "summary", ColNumInFile: 5},
			{TableColumn: "source", Column: "source", ColNumInFile: 6},
			{TableColumn: "created_at", Column: "created_at", ColNumInFile: 7},
		})
	s.importCSV(ctx, s.tableIDs["daily_summaries"], sumBuf.String(), "import_summaries.csv",
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "member_id", Column: "member_id", ColNumInFile: 2},
			{TableColumn: "daily_date", Column: "daily_date", ColNumInFile: 3},
			{TableColumn: "summary", Column: "summary", ColNumInFile: 4},
			{TableColumn: "status", Column: "status", ColNumInFile: 5},
			{TableColumn: "risk", Column: "risk", ColNumInFile: 6},
			{TableColumn: "blocker", Column: "blocker", ColNumInFile: 7},
		})
}

func (s *CatalogSync) SyncMembers(ctx context.Context, members []MemberRow) {
	if !s.ready {
		logger.Warn("catalog sync: skipped, not ready")
		return
	}
	var buf bytes.Buffer
	for _, m := range members {
		fmt.Fprintf(&buf, "%d,%s,,%s,,,%s\n", m.ID, m.Username, esc(m.Name), esc(m.Team))
	}
	s.importCSV(ctx, s.tableIDs["members"], buf.String(), "members.csv",
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "username", Column: "username", ColNumInFile: 2},
			{TableColumn: "password", Column: "password", ColNumInFile: 3},
			{TableColumn: "name", Column: "name", ColNumInFile: 4},
			{TableColumn: "avatar", Column: "avatar", ColNumInFile: 5},
			{TableColumn: "role", Column: "role", ColNumInFile: 6},
			{TableColumn: "team", Column: "team", ColNumInFile: 7},
		})
}

type MemberRow struct {
	ID       int
	Username string
	Name     string
	Team     string
}

func (s *CatalogSync) SyncAllMembers(members []model.Member) {
	var rows []MemberRow
	for _, m := range members {
		rows = append(rows, MemberRow{ID: m.ID, Username: m.Username, Name: m.Name, Team: m.Team})
	}
	s.SyncMembers(context.Background(), rows)
}

// SyncAllTeams 全量同步 teams 到 Catalog
func (s *CatalogSync) SyncAllTeams(teams []model.Team) {
	if !s.ready || len(teams) == 0 {
		return
	}
	tableID, ok := s.tableIDs["teams"]
	if !ok {
		return
	}
	logger.Info("catalog sync: full teams sync", "count", len(teams))
	var buf bytes.Buffer
	for _, t := range teams {
		fmt.Fprintf(&buf, "%d,%s\n", t.ID, esc(t.Name))
	}
	s.importCSV(context.Background(), tableID, buf.String(), "teams.csv",
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "name", Column: "name", ColNumInFile: 2},
		})
}

// SyncAllEntries 全量同步 daily_entries 到 Catalog
func (s *CatalogSync) SyncAllEntries(entries []model.DailyEntry) {
	logger.Info("catalog sync: full entries sync", "count", len(entries))
	s.SyncDailyEntries(context.Background(), entries)
}

func (s *CatalogSync) SyncAllSummaries(summaries []model.DailySummary) {
	if !s.ready || len(summaries) == 0 {
		return
	}
	logger.Info("catalog sync: full summaries sync", "count", len(summaries))
	var buf bytes.Buffer
	for _, sm := range summaries {
		fmt.Fprintf(&buf, "%d,%d,%s,%s,%s,%s,%s\n",
			sm.ID, sm.MemberID, dateOnly(sm.DailyDate), esc(sm.Summary), esc(sm.Status), esc(sm.Risk), esc(sm.Blocker))
	}
	s.importCSV(context.Background(), s.tableIDs["daily_summaries"], buf.String(), "all_summaries.csv",
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "member_id", Column: "member_id", ColNumInFile: 2},
			{TableColumn: "daily_date", Column: "daily_date", ColNumInFile: 3},
			{TableColumn: "summary", Column: "summary", ColNumInFile: 4},
			{TableColumn: "status", Column: "status", ColNumInFile: 5},
			{TableColumn: "risk", Column: "risk", ColNumInFile: 6},
			{TableColumn: "blocker", Column: "blocker", ColNumInFile: 7},
		})
}

func (s *CatalogSync) SyncAllTopicActivities(items []model.TopicActivity) {
	if !s.ready || len(items) == 0 {
		return
	}
	logger.Info("catalog sync: full topic_activities sync", "count", len(items))
	s.SyncTopicActivities(context.Background(), items)
}

func (s *CatalogSync) SyncAllTopics(topics []model.Topic) {
	if !s.ready || len(topics) == 0 {
		return
	}
	tableID, ok := s.tableIDs["topics"]
	if !ok {
		return
	}
	logger.Info("catalog sync: full topics sync", "count", len(topics))
	var buf bytes.Buffer
	for _, t := range topics {
		resolved := ""
		if t.ResolvedAt != nil {
			resolved = t.ResolvedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(&buf, "%d,%s,%s,%s,%s,%s\n",
			t.ID, esc(t.Name), esc(t.Description), t.Status, t.CreatedAt.Format("2006-01-02 15:04:05"), resolved)
	}
	s.importCSV(context.Background(), tableID, buf.String(), "topics.csv",
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "name", Column: "name", ColNumInFile: 2},
			{TableColumn: "description", Column: "description", ColNumInFile: 3},
			{TableColumn: "status", Column: "status", ColNumInFile: 4},
			{TableColumn: "created_at", Column: "created_at", ColNumInFile: 5},
			{TableColumn: "resolved_at", Column: "resolved_at", ColNumInFile: 6},
		})
}

func (s *CatalogSync) SyncTopicActivities(ctx context.Context, items []model.TopicActivity) {
	if !s.ready || len(items) == 0 {
		return
	}
	tableID, ok := s.tableIDs["topic_activities"]
	if !ok {
		return
	}
	var buf bytes.Buffer
	for _, a := range items {
		fmt.Fprintf(&buf, "%d,%s,%d,%s,%s,%s,%d\n",
			a.ID, esc(a.Topic), a.MemberID, esc(a.MemberName), dateOnly(a.DailyDate), esc(a.Content), a.EntryID)
	}
	s.importCSV(ctx, tableID, buf.String(), "topic_activities.csv",
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "topic", Column: "topic", ColNumInFile: 2},
			{TableColumn: "member_id", Column: "member_id", ColNumInFile: 3},
			{TableColumn: "member_name", Column: "member_name", ColNumInFile: 4},
			{TableColumn: "daily_date", Column: "daily_date", ColNumInFile: 5},
			{TableColumn: "content", Column: "content", ColNumInFile: 6},
			{TableColumn: "entry_id", Column: "entry_id", ColNumInFile: 7},
		})
}

func (s *CatalogSync) importCSV(ctx context.Context, tableID sdk.TableID, csv, fileName string, mapping []sdk.FileAndTableColumnMapping) {
	resp, err := s.raw.UploadLocalFile(ctx, bytes.NewReader([]byte(csv)), fileName, []sdk.FileMeta{{Filename: fileName, Path: "/"}})
	if err != nil {
		logger.Warn("catalog sync: upload failed", "table", tableID, "err", err)
		return
	}
	if len(resp.ConnFileIds) == 0 {
		logger.Warn("catalog sync: no conn_file_ids", "table", tableID)
		return
	}
	logger.Info("catalog sync: uploaded", "table", tableID, "file", fileName, "conn_file_ids", resp.ConnFileIds, "csv_len", len(csv))

	importResp, err := s.sdk.ImportLocalFileToTable(ctx, &sdk.TableConfig{
		ConnFileIDs:      resp.ConnFileIds,
		NewTable:         false,
		DatabaseID:       s.databaseID,
		TableID:          tableID,
		IsColumnName:     false,
		RowStart:         1,
		Conflict:         sdk.ConflictPolicyReplace,
		ExistedTable:     mapping,
		ExistedTableOpts: sdk.ExistedTableOptions{Method: sdk.ExistedTableOptionAppend},
	})
	if err != nil {
		logger.Warn("catalog sync: import failed", "table", tableID, "err", err)
		return
	}

	logger.Info("catalog sync: task submitted", "table", tableID, "file", fileName, "task_id", importResp.TaskId)
	if importResp.TaskId != 0 {
		go s.pollTask(tableID, fileName, importResp.TaskId)
	}
}

// pollTask polls task status via direct HTTP call.
// SDK's GetTask has a bug: doesn't handle {"task":{...}} wrapper and status is int not string.
func (s *CatalogSync) pollTask(tableID sdk.TableID, fileName string, taskID int64) {
	type envelope struct {
		Code string `json:"code"`
		Data struct {
			Task struct {
				Status      int `json:"status"`
				LoadResults []struct {
					Lines  int64  `json:"lines"`
					Reason string `json:"reason"`
				} `json:"load_results"`
			} `json:"task"`
		} `json:"data"`
	}
	url := fmt.Sprintf("%s/task/get?task_id=%d", s.baseURL, taskID)
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("moi-key", s.apiKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Warn("catalog sync: poll failed", "table", tableID, "task", taskID, "err", err)
			return
		}
		var env envelope
		json.NewDecoder(resp.Body).Decode(&env)
		resp.Body.Close()

		switch env.Data.Task.Status {
		case 4: // Finished
			lines := int64(0)
			for _, r := range env.Data.Task.LoadResults {
				lines += r.Lines
			}
			logger.Info("catalog sync: task done", "table", tableID, "file", fileName, "task", taskID, "lines", lines)
			return
		}
	}
	logger.Warn("catalog sync: task poll timeout", "table", tableID, "task", taskID)
}

func esc(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// dateOnly strips time portion from date strings like "2026-01-21T00:00:00Z" → "2026-01-21"
func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
