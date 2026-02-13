package service

import (
	"context"
	"fmt"
	"smart-daily/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

type DailyService struct {
	db          *gorm.DB
	lastEntryID int
}

func NewDailyService(db *gorm.DB) *DailyService { return &DailyService{db: db} }

func (s *DailyService) LastInsertID() int { return s.lastEntryID }

func (s *DailyService) Save(ctx context.Context, memberID int, date, content, summary, risk string) error {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	entry := model.DailyEntry{
		MemberID: memberID, DailyDate: date,
		Content: content, Summary: summary, Source: "chat",
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		return fmt.Errorf("insert entry: %w", err)
	}
	s.lastEntryID = entry.ID

	var existing model.DailySummary
	err := s.db.WithContext(ctx).
		Where("member_id = ? AND daily_date = ?", memberID, date).
		First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		return s.db.WithContext(ctx).Create(&model.DailySummary{
			MemberID: memberID, DailyDate: date, Summary: summary, Risk: risk,
		}).Error
	}
	if err != nil {
		return fmt.Errorf("query summary: %w", err)
	}

	return s.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"summary": existing.Summary + "\n" + summary,
		"risk":    risk,
	}).Error
}

func (s *DailyService) GetMemberWeekData(ctx context.Context, memberID int) (string, error) {
	var entries []model.DailyEntry
	err := s.db.WithContext(ctx).
		Where("member_id = ? AND daily_date >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)", memberID).
		Order("daily_date").Find(&entries).Error
	if err != nil {
		return "", fmt.Errorf("query member week: %w", err)
	}

	var sb strings.Builder
	for _, e := range entries {
		content := e.Summary
		if content == "" {
			content = e.Content
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", e.DailyDate, content))
	}
	if sb.Len() == 0 {
		return "本周暂无日报数据", nil
	}
	return sb.String(), nil
}
