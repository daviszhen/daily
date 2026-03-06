package repository

import (
	"context"
	"fmt"
	"smart-daily/internal/model"

	"gorm.io/gorm"
)

type DailyRepo struct{ db *gorm.DB }

func NewDailyRepo(db *gorm.DB) *DailyRepo { return &DailyRepo{db: db} }

// SummaryRow is a flattened row for export.
type SummaryRow struct {
	DailyDate string
	Name      string
	Summary   string
	Risk      string
}

// ListSummariesWithMembers returns all summaries joined with active members, for export.
func (r *DailyRepo) ListSummariesWithMembers(ctx context.Context) ([]SummaryRow, error) {
	var rows []SummaryRow
	err := r.db.WithContext(ctx).Model(&model.DailySummary{}).
		Select("daily_summaries.daily_date, members.name, daily_summaries.summary, daily_summaries.risk").
		Joins("JOIN members ON members.id = daily_summaries.member_id").
		Scopes(model.ActiveMembers).
		Order("daily_summaries.daily_date DESC, members.name").
		Scan(&rows).Error
	return rows, err
}

// CreateEntry inserts a daily entry.
func (r *DailyRepo) CreateEntry(ctx context.Context, e *model.DailyEntry) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// UpsertSummary creates or updates a daily summary for a member+date.
func (r *DailyRepo) UpsertSummary(ctx context.Context, memberID int, date, summary, risk string) error {
	var existing model.DailySummary
	err := r.db.WithContext(ctx).Where("member_id = ? AND daily_date = ?", memberID, date).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.WithContext(ctx).Create(&model.DailySummary{
			MemberID: memberID, DailyDate: date, Summary: summary, Risk: risk,
		}).Error
	}
	if err != nil {
		return fmt.Errorf("query summary: %w", err)
	}
	return r.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"summary": summary, "risk": risk,
	}).Error
}

// GetSummary returns a summary for a member+date.
func (r *DailyRepo) GetSummary(ctx context.Context, memberID int, date string) (*model.DailySummary, error) {
	var s model.DailySummary
	err := r.db.WithContext(ctx).Where("member_id = ? AND daily_date = ?", memberID, date).First(&s).Error
	return &s, err
}

// GetMemberEntries returns entries for a member in a date range (or last 7 days if empty).
func (r *DailyRepo) GetDayEntries(ctx context.Context, memberID int, date string) ([]model.DailyEntry, error) {
	var entries []model.DailyEntry
	err := r.db.WithContext(ctx).Where("member_id = ? AND daily_date = ?", memberID, date).Order("created_at").Find(&entries).Error
	return entries, err
}

func (r *DailyRepo) GetMemberEntries(ctx context.Context, memberID int, start, end string) ([]model.DailyEntry, error) {
	var entries []model.DailyEntry
	q := r.db.WithContext(ctx).Where("member_id = ?", memberID)
	if start != "" && end != "" {
		q = q.Where("daily_date BETWEEN ? AND ?", start, end)
	} else {
		q = q.Where("daily_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)")
	}
	err := q.Order("daily_date").Find(&entries).Error
	return entries, err
}

// BulkReplaceImportEntries deletes existing import entries for given keys, then bulk creates new ones.
func (r *DailyRepo) BulkReplaceImportEntries(ctx context.Context, delKeys [][]interface{}, entries []model.DailyEntry) error {
	if len(delKeys) > 0 {
		r.db.WithContext(ctx).Where("source = 'import' AND (member_id, daily_date) IN ?", delKeys).Delete(&model.DailyEntry{})
	}
	if len(entries) > 0 {
		return r.db.WithContext(ctx).Create(&entries).Error
	}
	return nil
}

// FindExistingImportKeys returns member_id+daily_date pairs for existing import entries.
func (r *DailyRepo) FindExistingImportKeys(ctx context.Context) (map[[2]interface{}]bool, error) {
	var existing []model.DailyEntry
	if err := r.db.WithContext(ctx).Where("source = 'import'").Select("member_id, daily_date").Find(&existing).Error; err != nil {
		return nil, err
	}
	m := make(map[[2]interface{}]bool, len(existing))
	for _, e := range existing {
		m[[2]interface{}{e.MemberID, e.DailyDate}] = true
	}
	return m, nil
}

// SubmittedDates returns the set of dates with daily data (summaries or entries) for a member in a date range.
func (r *DailyRepo) SubmittedDates(ctx context.Context, memberID int, start, end string) (map[string]bool, error) {
	m := make(map[string]bool)
	var dates []string
	// Check summaries
	r.db.WithContext(ctx).Model(&model.DailySummary{}).
		Where("member_id = ? AND daily_date BETWEEN ? AND ?", memberID, start, end).
		Pluck("daily_date", &dates)
	for _, d := range dates {
		if len(d) >= 10 {
			m[d[:10]] = true
		}
	}
	// Also check entries (imports may not have summaries)
	dates = nil
	r.db.WithContext(ctx).Model(&model.DailyEntry{}).
		Where("member_id = ? AND daily_date BETWEEN ? AND ?", memberID, start, end).
		Distinct("daily_date").Pluck("daily_date", &dates)
	for _, d := range dates {
		if len(d) >= 10 {
			m[d[:10]] = true
		}
	}
	return m, nil
}
