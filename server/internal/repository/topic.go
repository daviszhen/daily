package repository

import (
	"context"
	"smart-daily/internal/model"
	"time"

	"gorm.io/gorm"
)

type TopicRepo struct{ db *gorm.DB }

func NewTopicRepo(db *gorm.DB) *TopicRepo { return &TopicRepo{db: db} }

// --- topic_activities ---

func (r *TopicRepo) BatchCreate(ctx context.Context, items []model.TopicActivity) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&items).Error
}

func (r *TopicRepo) DeleteByEntryID(ctx context.Context, entryID int) error {
	return r.db.WithContext(ctx).Where("entry_id = ?", entryID).Delete(&model.TopicActivity{}).Error
}

func (r *TopicRepo) ListDistinctTopics(ctx context.Context) ([]string, error) {
	var topics []string
	err := r.db.WithContext(ctx).Model(&model.TopicActivity{}).Distinct("topic").Pluck("topic", &topics).Error
	return topics, err
}

func (r *TopicRepo) ListByDateRange(ctx context.Context, start, end string) ([]model.TopicActivity, error) {
	var items []model.TopicActivity
	err := r.db.WithContext(ctx).Where("daily_date BETWEEN ? AND ?", start, end).
		Order("topic, daily_date DESC").Find(&items).Error
	return items, err
}

// ListByMemberAndDateRange returns summaries for active members in a date range.
func (r *TopicRepo) ListSummariesByDateRange(ctx context.Context, start, end string) ([]MemberDailySummary, error) {
	var rows []MemberDailySummary
	err := r.db.WithContext(ctx).Model(&model.DailySummary{}).
		Select("daily_summaries.member_id, members.name as member_name, daily_summaries.daily_date, daily_summaries.summary, daily_summaries.risk").
		Joins("JOIN members ON members.id = daily_summaries.member_id").
		Scopes(model.ActiveMembers).
		Where("daily_summaries.daily_date BETWEEN ? AND ?", start, end).
		Order("members.name, daily_summaries.daily_date DESC").
		Scan(&rows).Error
	return rows, err
}

type MemberDailySummary struct {
	MemberID   int    `json:"member_id"`
	MemberName string `json:"member_name"`
	DailyDate  string `json:"daily_date"`
	Summary    string `json:"summary"`
	Risk       string `json:"risk"`
}

// MergeTopicActivities renames all activities from oldTopic to newTopic.
func (r *TopicRepo) MergeTopicActivities(ctx context.Context, oldTopic, newTopic string) error {
	return r.db.WithContext(ctx).Model(&model.TopicActivity{}).
		Where("topic = ?", oldTopic).Update("topic", newTopic).Error
}

// --- topics registry ---

func (r *TopicRepo) EnsureTopics(ctx context.Context, names []string) {
	for _, name := range names {
		r.db.WithContext(ctx).Where("name = ?", name).
			FirstOrCreate(&model.Topic{Name: name, Status: "active"})
	}
}

func (r *TopicRepo) ListAllTopics(ctx context.Context) ([]model.Topic, error) {
	var topics []model.Topic
	err := r.db.WithContext(ctx).Order("CASE WHEN status = 'active' THEN 0 ELSE 1 END, name").Find(&topics).Error
	return topics, err
}

func (r *TopicRepo) ListActiveTopicNames(ctx context.Context) ([]string, error) {
	var names []string
	err := r.db.WithContext(ctx).Model(&model.Topic{}).Where("status = 'active'").Pluck("name", &names).Error
	return names, err
}

func (r *TopicRepo) UpdateTopic(ctx context.Context, id int, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Topic{}).Where("id = ?", id).Updates(updates).Error
}

func (r *TopicRepo) ResolveTopic(ctx context.Context, id int) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.Topic{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "resolved", "resolved_at": &now}).Error
}

func (r *TopicRepo) ReopenTopic(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Model(&model.Topic{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "active", "resolved_at": nil}).Error
}

func (r *TopicRepo) DeleteTopic(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Topic{}).Error
}

func (r *TopicRepo) MergeTopic(ctx context.Context, sourceID int, targetName string) error {
	var source model.Topic
	if err := r.db.WithContext(ctx).First(&source, sourceID).Error; err != nil {
		return err
	}
	// Rename all activities
	if err := r.MergeTopicActivities(ctx, source.Name, targetName); err != nil {
		return err
	}
	// Delete source topic
	return r.db.WithContext(ctx).Delete(&source).Error
}

// --- insights ---

type TopicInsight struct {
	TopicID   int    `json:"topic_id"`
	Topic     string `json:"topic"`
	FirstDate string `json:"first_date"`
	LastDate  string `json:"last_date"`
	Days      int    `json:"days"`
	MemberCnt int    `json:"member_count"`
	EntryCnt  int    `json:"entry_count"`
}

// ListInsights returns aggregated stats for active topics (only recently active ones).
func (r *TopicRepo) ListInsights(ctx context.Context) ([]TopicInsight, error) {
	var results []TopicInsight
	cutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
	err := r.db.WithContext(ctx).Model(&model.TopicActivity{}).
		Select("topics.id as topic_id, topic_activities.topic, MIN(topic_activities.daily_date) as first_date, MAX(topic_activities.daily_date) as last_date, COUNT(DISTINCT topic_activities.daily_date) as days, COUNT(DISTINCT topic_activities.member_id) as member_cnt, COUNT(*) as entry_cnt").
		Joins("JOIN topics ON topics.name = topic_activities.topic AND topics.status = 'active'").
		Where("topic_activities.daily_date >= ?", cutoff).
		Group("topics.id, topic_activities.topic").
		Order("days DESC, member_cnt DESC").
		Find(&results).Error
	return results, err
}

// ListTopicRisks returns risk items associated with active topics (last 90 days).
func (r *TopicRepo) ListTopicRisks(ctx context.Context) ([]TopicRiskItem, error) {
	var results []TopicRiskItem
	cutoff := time.Now().AddDate(0, 0, -90).Format("2006-01-02")
	err := r.db.WithContext(ctx).Model(&model.TopicActivity{}).
		Select("topic_activities.topic, topic_activities.member_name, topic_activities.daily_date, daily_summaries.risk").
		Joins("JOIN topics ON topics.name = topic_activities.topic AND topics.status = 'active'").
		Joins("JOIN daily_summaries ON daily_summaries.member_id = topic_activities.member_id AND daily_summaries.daily_date = topic_activities.daily_date").
		Where("daily_summaries.risk != '' AND daily_summaries.risk IS NOT NULL AND topic_activities.daily_date >= ?", cutoff).
		Order("topic_activities.daily_date DESC").
		Scan(&results).Error
	return results, err
}

type TopicRiskItem struct {
	Topic      string `json:"topic"`
	MemberName string `json:"member_name"`
	DailyDate  string `json:"daily_date"`
	Risk       string `json:"risk"`
}
