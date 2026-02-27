package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

// SessionService wraps MOI llm-proxy Session & Message APIs.
type SessionService struct {
	baseURL string // e.g. https://freetier-01...
	apiKey  string
	client  *http.Client
}

func NewSessionService(baseURL, apiKey string) *SessionService {
	return &SessionService{baseURL: baseURL, apiKey: apiKey, client: &http.Client{}}
}

const sessionSource = "smart-daily"

// --- Models (match llm-proxy API) ---

type Session struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	UserID    string `json:"user_id"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type ChatMessage struct {
	ID              int64  `json:"id"`
	SessionID       *int64 `json:"session_id"`
	Role            string `json:"role"`
	Content         string `json:"content"`
	Response        string `json:"response"`
	Config          string `json:"config"`
	OriginalContent string `json:"original_content"`
	Source          string `json:"source"`
	UserID          string `json:"user_id"`
	Model           string `json:"model"`
	Status          string `json:"status"`
	CreatedAt       int64  `json:"created_at"`
}

// --- API Methods ---

func (s *SessionService) CreateSession(ctx context.Context, userID, title string) (*Session, error) {
	body := map[string]string{"title": title, "source": sessionSource, "user_id": userID}
	var resp Session
	if err := s.doJSON(ctx, "POST", "/api/sessions", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *SessionService) ListSessions(ctx context.Context, userID string) ([]Session, error) {
	q := url.Values{"user_id": {userID}, "source": {sessionSource}, "page_size": {"50"}}
	var resp struct {
		Sessions []Session `json:"sessions"`
	}
	if err := s.doJSON(ctx, "GET", "/api/sessions?"+q.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Sessions, nil
}

func (s *SessionService) DeleteSession(ctx context.Context, sessionID int64) error {
	return s.doJSON(ctx, "DELETE", fmt.Sprintf("/api/sessions/%d", sessionID), nil, nil)
}

func (s *SessionService) ListMessages(ctx context.Context, sessionID int64) ([]ChatMessage, error) {
	// List returns IDs only (no content/config), so fetch each message in parallel.
	var stubs []ChatMessage
	path := fmt.Sprintf("/api/sessions/%d/messages?limit=200", sessionID)
	if err := s.doJSON(ctx, "GET", path, nil, &stubs); err != nil {
		return nil, err
	}
	if len(stubs) == 0 {
		return nil, nil
	}

	msgs := make([]ChatMessage, len(stubs))
	var wg sync.WaitGroup
	for i, stub := range stubs {
		wg.Add(1)
		go func(i int, id int64) {
			defer wg.Done()
			var m ChatMessage
			if err := s.doJSON(ctx, "GET", fmt.Sprintf("/api/chat-messages/%d", id), nil, &m); err == nil {
				msgs[i] = m
			} else {
				msgs[i] = stubs[i] // fallback to stub
			}
		}(i, stub.ID)
	}
	wg.Wait()
	return msgs, nil
}

func (s *SessionService) SaveMessage(ctx context.Context, userID string, sessionID int64, role, content, config string) error {
	body := map[string]interface{}{
		"user_id":    userID,
		"session_id": sessionID,
		"source":     sessionSource,
		"role":       role,
		"content":    content,
		"config":     config,
		"model":      "qwen-plus",
		"status":     "success",
	}
	return s.doJSON(ctx, "POST", "/api/chat-messages", body, nil)
}

// --- HTTP helper ---

func (s *SessionService) doJSON(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+"/llm-proxy"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("moi-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("session api %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("session api %s %s: status %d: %s", method, path, resp.StatusCode, data)
	}

	if out != nil {
		data, _ := io.ReadAll(resp.Body)
		if len(data) > 0 {
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
		}
	}
	return nil
}

// SessionIDStr converts int64 to string for JSON transport.
func SessionIDStr(id int64) string { return strconv.FormatInt(id, 10) }
