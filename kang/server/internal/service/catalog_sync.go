package service

import (
	"bytes"
	"context"
	"fmt"
	"smart-daily/internal/logger"
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
}

func NewCatalogSync(raw *sdk.RawClient, catalogID int64, dbName string) *CatalogSync {
	s := &CatalogSync{
		raw:      raw,
		sdk:      sdk.NewSDKClient(raw),
		tableIDs: make(map[string]sdk.TableID),
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

	// 2. find tables by name
	children, err := s.raw.GetDatabaseChildren(ctx, &sdk.DatabaseChildrenRequest{DatabaseID: s.databaseID})
	if err != nil {
		logger.Warn("catalog: list tables failed", "err", err)
		return
	}
	need := map[string]bool{"members": true, "daily_entries": true, "daily_summaries": true}
	for _, child := range children.List {
		if child.Typ == "table" && need[child.Name] {
			id, _ := strconv.ParseInt(child.ID, 10, 64)
			s.tableIDs[child.Name] = sdk.TableID(id)
		}
	}

	missing := []string{}
	for name := range need {
		if _, ok := s.tableIDs[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		logger.Warn("catalog: tables not found, please add them to Catalog manually", "missing", missing, "database_id", s.databaseID)
		return
	}

	s.ready = true
	logger.Info("catalog: discovered", "database_id", s.databaseID, "tables", s.tableIDs)
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
		entryID, memberID, date, esc(content), esc(summary), now)
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
		entryID, memberID, date, esc(summary), esc(risk))
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

func (s *CatalogSync) SyncAllMembers(db *gorm.DB) {
	var rows []MemberRow
	if err := db.Raw("SELECT id, username, name, team FROM members").Scan(&rows).Error; err != nil {
		logger.Warn("catalog sync: query members failed", "err", err)
		return
	}
	s.SyncMembers(context.Background(), rows)
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
		Conflict:         1,
		ExistedTable:     mapping,
		ExistedTableOpts: sdk.ExistedTableOptions{Method: sdk.ExistedTableOptionAppend},
	})
	if err != nil {
		logger.Warn("catalog sync: import failed", "table", tableID, "err", err)
		return
	}
	logger.Info("catalog sync: ok", "table", tableID, "file", fileName, "import_resp", fmt.Sprintf("%+v", importResp))
}

func esc(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
