package service

import (
	"context"
	"fmt"
	"smart-daily/internal/model"
	"smart-daily/internal/repository"
	"strings"
	"time"
)

type DailyService struct{ repo *repository.DailyRepo }

func NewDailyService(repo *repository.DailyRepo) *DailyService { return &DailyService{repo: repo} }

func (s *DailyService) Save(ctx context.Context, memberID int, date, content, summary, risk string) (int, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	entry := &model.DailyEntry{
		MemberID: memberID, DailyDate: date,
		Content: content, Summary: summary, Source: "chat",
	}
	if err := s.repo.CreateEntry(ctx, entry); err != nil {
		return 0, fmt.Errorf("insert entry: %w", err)
	}
	return entry.ID, nil
}

func (s *DailyService) UpdateDailySummary(ctx context.Context, memberID int, date, summary, risk string) error {
	return s.repo.UpsertSummary(ctx, memberID, date, summary, risk)
}

func (s *DailyService) GetDayEntries(ctx context.Context, memberID int, date string) ([]model.DailyEntry, error) {
	return s.repo.GetDayEntries(ctx, memberID, date)
}

func (s *DailyService) GetDailySummary(ctx context.Context, memberID int, date string, out *model.DailySummary) error {
	got, err := s.repo.GetSummary(ctx, memberID, date)
	if err != nil {
		return err
	}
	*out = *got
	return nil
}

func (s *DailyService) GetMemberWeekData(ctx context.Context, memberID int) (string, error) {
	return s.GetMemberDateRangeData(ctx, memberID, "", "")
}

func (s *DailyService) GetMemberDateRangeData(ctx context.Context, memberID int, start, end string) (string, error) {
	entries, err := s.repo.GetMemberEntries(ctx, memberID, start, end)
	if err != nil {
		return "", fmt.Errorf("query member data: %w", err)
	}
	var sb strings.Builder
	for _, e := range entries {
		content := e.Summary
		if content == "" {
			content = e.Content
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", e.DailyDate, content))
	}
	return sb.String(), nil
}
