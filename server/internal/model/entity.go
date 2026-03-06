package model

import (
	"time"

	"gorm.io/gorm"
)

type Team struct {
	ID   int    `gorm:"primaryKey" json:"id"`
	Name string `gorm:"uniqueIndex" json:"name"`
}

type Member struct {
	ID       int    `gorm:"primaryKey" json:"id"`
	Username string `gorm:"uniqueIndex" json:"username"`
	Password string `json:"-"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	Role     string `json:"role"`
	TeamID   int    `json:"team_id"`
	Team     string `json:"team"`
	Status   string `gorm:"default:active" json:"status"`
	IsAdmin  bool   `gorm:"default:false" json:"is_admin"`
}

type DailyEntry struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	MemberID  int       `json:"member_id"`
	DailyDate string    `gorm:"type:date" json:"daily_date"`
	Content   string    `json:"content"`
	Summary   string    `json:"summary"`
	Source    string    `gorm:"default:chat" json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

type DailySummary struct {
	ID        int    `gorm:"primaryKey" json:"id"`
	MemberID  int    `json:"member_id"`
	DailyDate string `gorm:"type:date;uniqueIndex:uk_member_date" json:"daily_date"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	Risk      string `json:"risk"`
	Blocker   string `json:"blocker"`
}

type TopicActivity struct {
	ID         int    `gorm:"primaryKey" json:"id"`
	Topic      string `gorm:"index" json:"topic"`
	MemberID   int    `json:"member_id"`
	MemberName string `json:"member_name"`
	DailyDate  string `gorm:"type:date;index" json:"daily_date"`
	Content    string `json:"content"`
	EntryID    int    `json:"entry_id"`
}

type Topic struct {
	ID          int        `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"uniqueIndex" json:"name"`
	Description string     `json:"description"`
	Status      string     `gorm:"default:active" json:"status"` // active / resolved
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

func (Team) TableName() string            { return "teams" }
func (DailySummary) TableName() string    { return "daily_summaries" }
func (DailyEntry) TableName() string      { return "daily_entries" }
func (Member) TableName() string          { return "members" }
func (TopicActivity) TableName() string   { return "topic_activities" }
func (Topic) TableName() string           { return "topics" }
func (Feedback) TableName() string        { return "feedback" }

type Feedback struct {
	ID         int       `gorm:"primaryKey" json:"id"`
	MemberID   int       `json:"member_id"`
	MemberName string    `json:"member_name"`
	Content    string    `json:"content"`
	Status     string    `gorm:"default:open" json:"status"` // open / closed
	CreatedAt  time.Time `json:"created_at"`
}

// ActiveMembers is a GORM scope that excludes logically deleted members.
// Use: db.Scopes(model.ActiveMembers).Find(&members)
func ActiveMembers(db *gorm.DB) *gorm.DB {
	return db.Where("members.status != 'deleted'")
}
