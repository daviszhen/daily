package model

import "time"

type Member struct {
	ID       int    `gorm:"primaryKey" json:"id"`
	Username string `gorm:"uniqueIndex" json:"username"`
	Password string `json:"-"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	Role     string `json:"role"`
	Team     string `json:"team"`
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

func (DailySummary) TableName() string { return "daily_summaries" }
func (DailyEntry) TableName() string   { return "daily_entries" }
func (Member) TableName() string       { return "members" }
