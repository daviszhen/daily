package model

type ChatRequest struct {
	Text      string        `json:"text"`
	Mode      string        `json:"mode"`
	Date      string        `json:"date"`
	Action    string        `json:"action"`
	SessionID *int64        `json:"session_id,omitempty"`
	History   []HistoryItem `json:"history,omitempty"`
}

type HistoryItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Content  string                 `json:"content"`
	Type     string                 `json:"type,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type User struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Role   string `json:"role"`
}

type PendingReport struct {
	Content  string
	Summary  string
	Risks    []string
	Date     string
	MemberID int
}
