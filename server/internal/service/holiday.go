package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"smart-daily/internal/logger"
	"sync"
	"time"
)

// HolidayService caches Chinese holiday/调休 data.
// Primary: api.apihubs.cn, Fallback: GitHub NateScarlet/holiday-cn.
type HolidayService struct {
	mu          sync.RWMutex
	data        map[string]holidayEntry // "2026-01-01" → entry
	loadedYears map[int]bool
}

type holidayEntry struct {
	Name     string
	IsOffDay bool
}

func NewHolidayService() *HolidayService {
	s := &HolidayService{data: make(map[string]holidayEntry), loadedYears: make(map[int]bool)}
	year := time.Now().Year()
	go func() {
		s.loadYear(year)
		s.loadYear(year - 1)
	}()
	return s
}

func (s *HolidayService) loadYear(year int) {
	s.mu.RLock()
	loaded := s.loadedYears[year]
	s.mu.RUnlock()
	if loaded {
		return
	}

	// Try apihubs.cn first, then GitHub
	if n := s.loadFromApihubs(year); n > 0 {
		logger.Info("holiday.loaded", "year", year, "entries", n, "source", "apihubs.cn")
		return
	}
	if n := s.loadFromGitHub(year); n > 0 {
		logger.Info("holiday.loaded", "year", year, "entries", n, "source", "github")
		return
	}
	logger.Warn("holiday.load.failed", "year", year)
	// Don't mark as loaded — will retry on next request
}

// --- apihubs.cn ---

type apihubsResp struct {
	Code int `json:"code"`
	Data struct {
		List []struct {
			Date       int    `json:"date"`
			Workday    int    `json:"workday"`
			HolidayCN  string `json:"holiday_or_cn"`
			OvertimeCN string `json:"holiday_overtime_cn"`
		} `json:"list"`
	} `json:"data"`
}

func (s *HolidayService) loadFromApihubs(year int) int {
	url := fmt.Sprintf("https://api.apihubs.cn/holiday/get?year=%d&size=366&cn=1&order_by=1", year)
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var result apihubsResp
	if json.NewDecoder(resp.Body).Decode(&result) != nil || result.Code != 0 {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, d := range result.Data.List {
		ds := fmt.Sprintf("%04d-%02d-%02d", d.Date/10000, (d.Date/100)%100, d.Date%100)
		if d.HolidayCN != "非节假日" {
			s.data[ds] = holidayEntry{Name: d.HolidayCN, IsOffDay: d.Workday == 2}
			count++
		} else if d.OvertimeCN != "非节假日调休" {
			s.data[ds] = holidayEntry{Name: d.OvertimeCN, IsOffDay: false}
			count++
		}
	}
	s.loadedYears[year] = true
	return count
}

// --- GitHub NateScarlet/holiday-cn ---

type githubResp struct {
	Days []struct {
		Date     string `json:"date"`
		Name     string `json:"name"`
		IsOffDay bool   `json:"isOffDay"`
	} `json:"days"`
}

func (s *HolidayService) loadFromGitHub(year int) int {
	url := fmt.Sprintf("https://cdn.jsdelivr.net/gh/NateScarlet/holiday-cn@master/%d.json", year)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Get(url)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return 0
	}
	defer resp.Body.Close()

	var result githubResp
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range result.Days {
		s.data[d.Date] = holidayEntry{Name: d.Name, IsOffDay: d.IsOffDay}
	}
	s.loadedYears[year] = true
	return len(result.Days)
}

func (s *HolidayService) markLoaded(year int) {
	s.mu.Lock()
	s.loadedYears[year] = true
	s.mu.Unlock()
}

// EnsureYear loads holiday data for a year if not already cached.
func (s *HolidayService) EnsureYear(year int) { s.loadYear(year) }

// DayInfo returns holiday info for a date. Returns (name, isOffDay, found).
func (s *HolidayService) DayInfo(date string) (string, bool, bool) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", false, false
	}
	s.loadYear(t.Year())
	s.mu.RLock()
	e, ok := s.data[date]
	s.mu.RUnlock()
	return e.Name, e.IsOffDay, ok
}

// IsWorkday returns true if the date is a workday (considering holidays and 调休).
func (s *HolidayService) IsWorkday(date string) bool {
	_, isOffDay, found := s.DayInfo(date)
	if found {
		return !isOffDay
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	wd := t.Weekday()
	return wd >= time.Monday && wd <= time.Friday
}
