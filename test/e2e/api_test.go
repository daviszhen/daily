package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// apiClient wraps HTTP calls with auth token.
type apiClient struct {
	t     *testing.T
	token string
}

func newAPIClient(t *testing.T) *apiClient {
	t.Helper()
	c := &apiClient{t: t}
	c.login("kuaiweikang", "123456")
	return c
}

func (c *apiClient) login(username, password string) {
	c.t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := http.Post(baseURL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		c.t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	token, ok := result["token"].(string)
	if !ok || token == "" {
		c.t.Fatalf("login failed: %v", result)
	}
	c.token = token
}

func (c *apiClient) do(method, path string, body interface{}) (int, map[string]interface{}) {
	c.t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, baseURL+path, reqBody)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return resp.StatusCode, result
}

func (c *apiClient) doList(method, path string) (int, []interface{}) {
	c.t.Helper()
	req, _ := http.NewRequest(method, baseURL+path, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer resp.Body.Close()
	var result []interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return resp.StatusCode, result
}

func (c *apiClient) doRaw(method, path string, body ...interface{}) *http.Response {
	c.t.Helper()
	var reqBody io.Reader
	if len(body) > 0 && body[0] != nil {
		b, _ := json.Marshal(body[0])
		reqBody = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, baseURL+path, reqBody)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s failed: %v", method, path, err)
	}
	return resp
}

// --- API Tests ---

func TestAPIMembers(t *testing.T) {
	c := newAPIClient(t)

	code, list := c.doList("GET", "/api/members")
	if code != 200 {
		t.Fatalf("GET /api/members: status %d", code)
	}
	if len(list) == 0 {
		t.Fatal("GET /api/members: empty list")
	}

	// Check structure
	first := list[0].(map[string]interface{})
	for _, field := range []string{"id", "name", "role", "status", "team_id", "team_name"} {
		if _, ok := first[field]; !ok {
			t.Errorf("member missing field: %s", field)
		}
	}

	// Deleted members should not appear
	for _, m := range list {
		member := m.(map[string]interface{})
		if member["status"] == "deleted" {
			t.Errorf("deleted member in list: %v", member["name"])
		}
	}
	t.Logf("OK: %d members returned with correct structure", len(list))
}

func TestAPITeams(t *testing.T) {
	c := newAPIClient(t)
	teamName := fmt.Sprintf("e2e测试组_%d", time.Now().UnixNano()%100000)

	// Create team
	code, result := c.do("POST", "/api/teams", map[string]string{"name": teamName})
	if code != 200 {
		t.Fatalf("POST /api/teams: status %d, %v", code, result)
	}
	teamID := result["id"].(float64)
	if teamID == 0 {
		t.Fatal("team id should not be 0")
	}
	t.Logf("OK: created team id=%.0f", teamID)

	// List teams
	code, list := c.doList("GET", "/api/teams")
	if code != 200 {
		t.Fatalf("GET /api/teams: status %d", code)
	}
	found := false
	for _, item := range list {
		team := item.(map[string]interface{})
		if team["name"] == teamName {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("created team not found in list")
	}
	t.Log("OK: team listed")

	// Duplicate should fail
	code, _ = c.do("POST", "/api/teams", map[string]string{"name": teamName})
	if code == 200 {
		t.Fatal("duplicate team creation should fail")
	}
	t.Log("OK: duplicate team rejected")
}

func TestAPIMemberUpdate(t *testing.T) {
	c := newAPIClient(t)

	// Get a member
	_, list := c.doList("GET", "/api/members")
	if len(list) == 0 {
		t.Fatal("no members")
	}
	member := list[0].(map[string]interface{})
	memberID := int(member["id"].(float64))

	// Update role
	code, result := c.do("PUT", fmt.Sprintf("/api/members/%d", memberID), map[string]string{"role": "Leader"})
	if code != 200 {
		t.Fatalf("PUT member role: status %d, %v", code, result)
	}

	// Verify
	_, list = c.doList("GET", "/api/members")
	for _, m := range list {
		mem := m.(map[string]interface{})
		if int(mem["id"].(float64)) == memberID {
			if mem["role"] != "Leader" {
				t.Fatalf("role not updated: got %v", mem["role"])
			}
			break
		}
	}
	t.Log("OK: member role updated")

	// Restore
	c.do("PUT", fmt.Sprintf("/api/members/%d", memberID), map[string]string{"role": "开发工程师"})
}

func TestAPIMemberDelete(t *testing.T) {
	c := newAPIClient(t)

	// Get test member (use a test account)
	_, list := c.doList("GET", "/api/members")
	var testMemberID int
	var testMemberName string
	for _, m := range list {
		mem := m.(map[string]interface{})
		name := mem["name"].(string)
		if len(name) >= 2 && name[:6] == "测试" {
			testMemberID = int(mem["id"].(float64))
			testMemberName = name
			break
		}
	}
	if testMemberID == 0 {
		t.Skip("no test member found, skipping delete test")
	}

	// Delete (logical)
	code, result := c.do("DELETE", fmt.Sprintf("/api/members/%d", testMemberID), nil)
	if code != 200 {
		t.Fatalf("DELETE member: status %d, %v", code, result)
	}

	// Verify not in list
	_, list = c.doList("GET", "/api/members")
	for _, m := range list {
		mem := m.(map[string]interface{})
		if int(mem["id"].(float64)) == testMemberID {
			t.Fatal("deleted member still in list")
		}
	}
	t.Logf("OK: member '%s' logically deleted, not in list", testMemberName)
}

func TestAPIExportDaily(t *testing.T) {
	c := newAPIClient(t)

	resp := c.doRaw("GET", "/api/export/daily")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("GET /api/export/daily: status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("wrong content-type: %s", ct)
	}

	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		t.Fatal("missing Content-Disposition header")
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) < 100 {
		t.Fatalf("xlsx too small: %d bytes", len(body))
	}

	// Check xlsx magic bytes (PK zip header)
	if body[0] != 0x50 || body[1] != 0x4B {
		t.Fatal("not a valid xlsx file (wrong magic bytes)")
	}

	t.Logf("OK: exported %d bytes xlsx", len(body))
}

func TestAPILoginFail(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"username": "nonexistent", "password": "wrong"})
	resp, err := http.Post(baseURL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("login with wrong credentials should fail")
	}
	t.Log("OK: invalid login rejected")
}

func TestAPIUnauthorized(t *testing.T) {
	// No token
	resp, err := http.Get(baseURL + "/api/members")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("should reject unauthenticated request")
	}
	t.Logf("OK: unauthorized request rejected with %d", resp.StatusCode)
}

// --- Topic & Feed API Tests ---

func TestAPIFeedByMember(t *testing.T) {
	c := newAPIClient(t)
	code, result := c.do("GET", "/api/feed/by-member?start=2024-01-01&end=2026-12-31", nil)
	if code != 200 {
		t.Fatalf("status %d, %v", code, result)
	}
	members, ok := result["members"].([]interface{})
	if !ok {
		t.Fatal("missing 'members' field")
	}
	if len(members) == 0 {
		t.Fatal("empty members feed")
	}
	first := members[0].(map[string]interface{})
	for _, f := range []string{"member_id", "member_name", "items"} {
		if _, ok := first[f]; !ok {
			t.Errorf("missing field: %s", f)
		}
	}
	t.Logf("OK: %d members in feed", len(members))
}

func TestAPIFeedByTopic(t *testing.T) {
	c := newAPIClient(t)
	code, result := c.do("GET", "/api/feed/by-topic?start=2024-01-01&end=2026-12-31", nil)
	if code != 200 {
		t.Fatalf("status %d, %v", code, result)
	}
	topics, ok := result["topics"].([]interface{})
	if !ok {
		t.Fatal("missing 'topics' field")
	}
	if len(topics) == 0 {
		t.Fatal("empty topics feed")
	}
	first := topics[0].(map[string]interface{})
	for _, f := range []string{"topic", "members", "items"} {
		if _, ok := first[f]; !ok {
			t.Errorf("missing field: %s", f)
		}
	}
	t.Logf("OK: %d topics in feed", len(topics))
}

func TestAPIInsights(t *testing.T) {
	c := newAPIClient(t)
	code, result := c.do("GET", "/api/insights", nil)
	if code != 200 {
		t.Fatalf("status %d, %v", code, result)
	}
	insights, ok := result["insights"].([]interface{})
	if !ok {
		t.Fatal("missing 'insights' field")
	}
	if len(insights) == 0 {
		t.Skip("no insights data yet")
	}
	first := insights[0].(map[string]interface{})
	for _, f := range []string{"topic_id", "topic", "days", "member_count", "risk_level"} {
		if _, ok := first[f]; !ok {
			t.Errorf("missing field: %s", f)
		}
	}
	// Verify risk_level values
	for _, item := range insights {
		ins := item.(map[string]interface{})
		level := ins["risk_level"].(string)
		if level != "high" && level != "medium" && level != "low" {
			t.Errorf("invalid risk_level: %s", level)
		}
	}
	t.Logf("OK: %d insights with valid risk levels", len(insights))
}

func TestAPITopicsList(t *testing.T) {
	c := newAPIClient(t)
	code, list := c.doList("GET", "/api/topics/all")
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	if len(list) == 0 {
		t.Skip("no topics yet")
	}
	first := list[0].(map[string]interface{})
	for _, f := range []string{"id", "name", "status"} {
		if _, ok := first[f]; !ok {
			t.Errorf("missing field: %s", f)
		}
	}
	t.Logf("OK: %d topics", len(list))
}

func TestAPITopicResolveAndReopen(t *testing.T) {
	c := newAPIClient(t)

	// Get a topic
	_, list := c.doList("GET", "/api/topics/all")
	if len(list) == 0 {
		t.Skip("no topics")
	}
	topic := list[0].(map[string]interface{})
	topicID := int(topic["id"].(float64))

	// Resolve
	code, _ := c.do("PUT", fmt.Sprintf("/api/topics/%d/resolve", topicID), nil)
	if code != 200 {
		t.Fatalf("resolve: status %d", code)
	}

	// Verify resolved
	_, list = c.doList("GET", "/api/topics/all")
	for _, item := range list {
		tp := item.(map[string]interface{})
		if int(tp["id"].(float64)) == topicID {
			if tp["status"] != "resolved" {
				t.Fatalf("expected resolved, got %v", tp["status"])
			}
			break
		}
	}
	t.Logf("OK: topic %d resolved", topicID)

	// Reopen
	code, _ = c.do("PUT", fmt.Sprintf("/api/topics/%d/reopen", topicID), nil)
	if code != 200 {
		t.Fatalf("reopen: status %d", code)
	}
	t.Logf("OK: topic %d reopened", topicID)
}

func TestAPITopicRename(t *testing.T) {
	c := newAPIClient(t)

	_, list := c.doList("GET", "/api/topics/all")
	if len(list) == 0 {
		t.Skip("no topics")
	}
	// Find a safe topic to rename (pick last one to avoid disrupting important ones)
	topic := list[len(list)-1].(map[string]interface{})
	topicID := int(topic["id"].(float64))
	origName := topic["name"].(string)

	// Rename
	newName := origName + "_e2e_test"
	code, _ := c.do("PUT", fmt.Sprintf("/api/topics/%d", topicID), map[string]string{"name": newName})
	if code != 200 {
		t.Fatalf("rename: status %d", code)
	}

	// Verify
	_, list = c.doList("GET", "/api/topics/all")
	found := false
	for _, item := range list {
		tp := item.(map[string]interface{})
		if int(tp["id"].(float64)) == topicID && tp["name"] == newName {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("renamed topic not found")
	}
	t.Logf("OK: topic %d renamed to %s", topicID, newName)

	// Restore
	c.do("PUT", fmt.Sprintf("/api/topics/%d", topicID), map[string]string{"name": origName})
}

func TestAPICalendar(t *testing.T) {
	c := newAPIClient(t)

	// Current month
	code, result := c.do("GET", "/api/calendar?month=2026-03", nil)
	if code != 200 {
		t.Fatalf("status %d, %v", code, result)
	}
	days, ok := result["days"].([]interface{})
	if !ok || len(days) == 0 {
		t.Fatal("missing or empty days")
	}
	// March has 31 days
	if len(days) != 31 {
		t.Fatalf("expected 31 days, got %d", len(days))
	}
	first := days[0].(map[string]interface{})
	for _, f := range []string{"date", "weekday", "is_workday", "submitted"} {
		if _, ok := first[f]; !ok {
			t.Errorf("missing field: %s", f)
		}
	}
	t.Logf("OK: 2026-03 has %d days, workdays=%.0f, filled=%.0f", len(days), result["workdays"], result["filled_workdays"])

	// Navigate to previous month (翻页)
	code, result = c.do("GET", "/api/calendar?month=2026-02", nil)
	if code != 200 {
		t.Fatalf("2026-02: status %d", code)
	}
	days = result["days"].([]interface{})
	if len(days) != 28 {
		t.Fatalf("Feb 2026 expected 28 days, got %d", len(days))
	}
	t.Logf("OK: 2026-02 has %d days", len(days))

	// Cross-year navigation
	code, result = c.do("GET", "/api/calendar?month=2025-12", nil)
	if code != 200 {
		t.Fatalf("2025-12: status %d", code)
	}
	days = result["days"].([]interface{})
	if len(days) != 31 {
		t.Fatalf("Dec 2025 expected 31 days, got %d", len(days))
	}
	t.Logf("OK: 2025-12 cross-year navigation works, %d days", len(days))

	// Holiday data: Jan 2026 should have 元旦
	code, result = c.do("GET", "/api/calendar?month=2026-01", nil)
	if code != 200 {
		t.Fatalf("2026-01: status %d", code)
	}
	days = result["days"].([]interface{})
	jan1 := days[0].(map[string]interface{})
	if jan1["holiday"] == nil || jan1["holiday"] == "" {
		t.Error("Jan 1 should have holiday name")
	}
	if jan1["is_workday"] == true {
		t.Error("Jan 1 (元旦) should not be a workday")
	}
	// Jan 4 is 调休 (补班)
	jan4 := days[3].(map[string]interface{})
	if jan4["is_workday"] != true {
		t.Error("Jan 4 (调休补班) should be a workday")
	}
	t.Logf("OK: 2026-01 holidays correct, Jan1=%v Jan4_workday=%v", jan1["holiday"], jan4["is_workday"])
}

func TestAPILogs(t *testing.T) {
	c := newAPIClient(t)

	// Admin can read logs
	resp := c.doRaw("GET", "/api/logs?lines=5")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	lines := bytes.Count(body, []byte("\n"))
	if lines == 0 {
		t.Fatal("expected log lines")
	}
	t.Logf("OK: got %d lines", lines)

	// Download mode
	resp2 := c.doRaw("GET", "/api/logs?download=true")
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("download status %d", resp2.StatusCode)
	}
	if ct := resp2.Header.Get("Content-Disposition"); ct == "" {
		t.Error("missing Content-Disposition header")
	}
	t.Log("OK: download mode works")

	// Non-admin should be rejected — use request without token
	req, _ := http.NewRequest("GET", baseURL+"/api/logs?lines=5", nil)
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 401 {
		t.Fatalf("unauthenticated expected 401, got %d", resp3.StatusCode)
	}
	t.Log("OK: unauthenticated rejected")
}

func TestBenchmarkReportValidate(t *testing.T) {
	benchData := loadBenchmarks(t)
	c := newAPIClient(t)

	for _, tc := range benchData.ReportValidate.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			start := time.Now()
			resp := c.doRaw("POST", "/api/chat/stream", map[string]string{"text": tc.Input, "mode": "report"})
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			elapsed := time.Since(start).Seconds()

			content := string(body)
			hasConfirm := strings.Contains(content, "summary_confirm")

			if tc.ExpectValid && tc.ExpectSufficient {
				if !hasConfirm {
					t.Errorf("expected summary_confirm for valid+sufficient input")
				}
			} else {
				if hasConfirm {
					t.Errorf("unexpected summary_confirm for invalid/insufficient input")
				}
			}

			if elapsed > tc.MaxSeconds {
				t.Logf("WARN: %s took %.1fs, exceeds baseline %.0fs", tc.Name, elapsed, tc.MaxSeconds)
			}
			t.Logf("OK: %s → %.1fs (limit %.0fs), hasConfirm=%v", tc.Name, elapsed, tc.MaxSeconds, hasConfirm)
		})
	}
}

func TestBenchmarkDataAsking(t *testing.T) {
	benchData := loadBenchmarks(t)
	c := newAPIClient(t)

	for _, tc := range benchData.DataAsking.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			start := time.Now()
			resp := c.doRaw("POST", "/api/chat/stream", map[string]string{"text": tc.Input, "mode": "query"})
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			elapsed := time.Since(start).Seconds()

			hasAnswer := strings.Contains(string(body), "\"token\"")
			if tc.ExpectHasAnswer && !hasAnswer {
				t.Errorf("expected answer tokens but got none")
			}

			if elapsed > tc.MaxSeconds {
				t.Logf("WARN: %s took %.1fs, exceeds baseline %.0fs", tc.Name, elapsed, tc.MaxSeconds)
			}
			t.Logf("OK: %s → %.1fs (limit %.0fs), hasAnswer=%v", tc.Name, elapsed, tc.MaxSeconds, hasAnswer)
		})
	}
}

func TestBenchmarkMergeSummary(t *testing.T) {
	benchData := loadBenchmarks(t)
	c := newAPIClient(t)

	for _, tc := range benchData.MergeSummary.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Submit each entry through stream → confirm flow (supplement mode with specific date)
			for i, entry := range tc.Entries {
				start := time.Now()
				resp := c.doRaw("POST", "/api/chat/stream", map[string]string{"text": entry, "mode": "supplement", "date": tc.Date})
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if !strings.Contains(string(body), "summary_confirm") {
					t.Fatalf("entry %d did not get confirm card: %s", i, string(body)[:min(len(body), 200)])
				}
				// Confirm
				code, result := c.do("POST", "/api/chat", map[string]string{"action": "confirm"})
				if code != 200 {
					t.Fatalf("confirm entry %d failed: %d %v", i, code, result)
				}
				t.Logf("entry %d submitted in %.1fs", i, time.Since(start).Seconds())
			}

			// Check merged summary via calendar/day
			resp := c.doRaw("GET", "/api/calendar/day?date="+tc.Date)
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			summary := string(body)

			for _, kw := range tc.ExpectContains {
				if !strings.Contains(summary, kw) {
					t.Errorf("merged summary missing expected keyword %q\nsummary: %s", kw, summary)
				}
			}
			for _, kw := range tc.ExpectNotContains {
				if strings.Contains(summary, kw) {
					t.Errorf("merged summary should NOT contain %q (was supposed to be discarded)\nsummary: %s", kw, summary)
				}
			}
			t.Logf("OK: %s → %s", tc.Name, summary[:min(len(summary), 200)])
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- benchmark helpers ---

type benchmarks struct {
	ReportValidate struct {
		Cases []struct {
			Name             string  `json:"name"`
			Input            string  `json:"input"`
			ExpectValid      bool    `json:"expect_valid"`
			ExpectSufficient bool    `json:"expect_sufficient"`
			MaxSeconds       float64 `json:"max_seconds"`
		} `json:"cases"`
	} `json:"report_validate"`
	DataAsking struct {
		Cases []struct {
			Name            string  `json:"name"`
			Input           string  `json:"input"`
			ExpectHasAnswer bool    `json:"expect_has_answer"`
			MaxSeconds      float64 `json:"max_seconds"`
		} `json:"cases"`
	} `json:"data_asking"`
	MergeSummary struct {
		Cases []struct {
			Name              string   `json:"name"`
			Date              string   `json:"date"`
			Entries           []string `json:"entries"`
			ExpectContains    []string `json:"expect_contains"`
			ExpectNotContains []string `json:"expect_not_contains"`
			MaxSeconds        float64  `json:"max_seconds"`
		} `json:"cases"`
	} `json:"merge_summary"`
}

func loadBenchmarks(t *testing.T) benchmarks {
	t.Helper()
	data, err := os.ReadFile("benchmarks.json")
	if err != nil {
		t.Fatalf("load benchmarks.json: %v", err)
	}
	var b benchmarks
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("parse benchmarks.json: %v", err)
	}
	return b
}

func TestFeedback(t *testing.T) {
	c := newAPIClient(t)

	// Submit
	code, fb := c.do("POST", "/api/feedback", map[string]string{"content": "测试反馈意见"})
	if code != 200 {
		t.Fatalf("submit failed: %d", code)
	}
	fbID := int(fb["id"].(float64))
	t.Log("OK: feedback submitted")

	// List
	code2, list := c.doList("GET", "/api/feedback")
	if code2 != 200 || len(list) == 0 {
		t.Fatal("feedback list empty")
	}
	t.Logf("OK: feedback list has %d items", len(list))

	// Close (admin)
	code3, _ := c.do("PUT", fmt.Sprintf("/api/feedback/%d/close", fbID), nil)
	if code3 != 200 {
		t.Fatalf("close failed: %d", code3)
	}
	t.Log("OK: feedback closed")

	// Delete (admin)
	resp4 := c.doRaw("DELETE", fmt.Sprintf("/api/feedback/%d", fbID))
	defer resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Fatalf("delete failed: %d", resp4.StatusCode)
	}
	t.Log("OK: feedback deleted")
}
