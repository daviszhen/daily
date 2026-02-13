package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	sdk "github.com/matrixorigin/moi-go-sdk"
)

const (
	catalogDatabaseID       = sdk.DatabaseID(12558898)
	catalogDailyEntriesID   = sdk.TableID(270002)
	catalogDailySummariesID = sdk.TableID(270003)
	catalogMembersID        = sdk.TableID(270001)
)

type CatalogSync struct {
	raw *sdk.RawClient
	sdk *sdk.SDKClient
}

func NewCatalogSync(raw *sdk.RawClient) *CatalogSync {
	return &CatalogSync{raw: raw, sdk: sdk.NewSDKClient(raw)}
}

func (s *CatalogSync) SyncDailySummary(ctx context.Context, entryID, memberID int, date, content, summary, risk string) {
	now := time.Now().Format("2006-01-02 15:04:05")

	// sync daily_entries (270002): id, member_id, daily_date, content, summary, source, created_at
	entryCsv := fmt.Sprintf("%d,%d,%s,%s,%s,chat,%s\n",
		entryID, memberID, date, esc(content), esc(summary), now)
	s.importCSV(ctx, catalogDailyEntriesID, entryCsv, fmt.Sprintf("entry_%d.csv", entryID),
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "member_id", Column: "member_id", ColNumInFile: 2},
			{TableColumn: "daily_date", Column: "daily_date", ColNumInFile: 3},
			{TableColumn: "content", Column: "content", ColNumInFile: 4},
			{TableColumn: "summary", Column: "summary", ColNumInFile: 5},
			{TableColumn: "source", Column: "source", ColNumInFile: 6},
			{TableColumn: "created_at", Column: "created_at", ColNumInFile: 7},
		})

	// sync daily_summaries (270003): id, member_id, daily_date, summary, status, risk, blocker
	sumCsv := fmt.Sprintf("%d,%d,%s,%s,,%s,\n",
		entryID, memberID, date, esc(summary), esc(risk))
	s.importCSV(ctx, catalogDailySummariesID, sumCsv, fmt.Sprintf("sum_%d.csv", entryID),
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
	var buf bytes.Buffer
	for _, m := range members {
		fmt.Fprintf(&buf, "%d,%s,,%s,%s\n", m.ID, m.Username, esc(m.Name), esc(m.Team))
	}
	s.importCSV(ctx, catalogMembersID, buf.String(), "members.csv",
		[]sdk.FileAndTableColumnMapping{
			{TableColumn: "id", Column: "id", ColNumInFile: 1},
			{TableColumn: "username", Column: "username", ColNumInFile: 2},
			{TableColumn: "password", Column: "password", ColNumInFile: 3},
			{TableColumn: "name", Column: "name", ColNumInFile: 4},
			{TableColumn: "team", Column: "team", ColNumInFile: 5},
		})
}

type MemberRow struct {
	ID       int
	Username string
	Name     string
	Team     string
}

func (s *CatalogSync) importCSV(ctx context.Context, tableID sdk.TableID, csv, fileName string, mapping []sdk.FileAndTableColumnMapping) {
	resp, err := s.raw.UploadLocalFile(ctx, bytes.NewReader([]byte(csv)), fileName, []sdk.FileMeta{{Filename: fileName, Path: "/"}})
	if err != nil {
		slog.Warn("catalog sync: upload failed", "table", tableID, "err", err)
		return
	}
	if len(resp.ConnFileIds) == 0 {
		slog.Warn("catalog sync: no conn_file_ids", "table", tableID)
		return
	}

	_, err = s.sdk.ImportLocalFileToTable(ctx, &sdk.TableConfig{
		ConnFileIDs:      resp.ConnFileIds,
		NewTable:         false,
		DatabaseID:       catalogDatabaseID,
		TableID:          tableID,
		IsColumnName:     false,
		RowStart:         1,
		Conflict:         1,
		ExistedTable:     mapping,
		ExistedTableOpts: sdk.ExistedTableOptions{Method: sdk.ExistedTableOptionAppend},
	})
	if err != nil {
		return
	}
	slog.Info("catalog sync: ok", "table", tableID, "file", fileName)
}

func esc(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
