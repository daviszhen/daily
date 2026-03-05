package handler

import (
	"fmt"
	"net/http"
	"smart-daily/internal/repository"
	"smart-daily/internal/service"
	"time"

	"github.com/gin-gonic/gin"
)

type CalendarHandler struct {
	daily   *repository.DailyRepo
	holiday *service.HolidayService
}

func NewCalendarHandler(daily *repository.DailyRepo, holiday *service.HolidayService) *CalendarHandler {
	return &CalendarHandler{daily: daily, holiday: holiday}
}

type calendarDay struct {
	Date      string `json:"date"`
	Weekday   int    `json:"weekday"` // 0=Sun..6=Sat
	IsWorkday bool   `json:"is_workday"`
	Holiday   string `json:"holiday,omitempty"` // e.g. "春节", "元旦"
	Submitted bool   `json:"submitted"`
}

// Calendar handles GET /api/calendar?month=2026-03
func (h *CalendarHandler) Calendar(c *gin.Context) {
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))
	start, err := time.Parse("2006-01", month)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid month format, use YYYY-MM"})
		return
	}
	end := start.AddDate(0, 1, -1) // last day of month
	today := time.Now().Format("2006-01-02")

	uid := c.GetInt("user_id")
	submitted, _ := h.daily.SubmittedDates(c.Request.Context(), uid, start.Format("2006-01-02"), end.Format("2006-01-02"))

	h.holiday.EnsureYear(start.Year())

	var days []calendarDay
	workdays, filledWorkdays := 0, 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		isWork := h.holiday.IsWorkday(ds)
		name, isOff, found := h.holiday.DayInfo(ds)
		holiday := ""
		if found {
			if isOff {
				holiday = name
			} else {
				holiday = fmt.Sprintf("%s(补班)", name)
			}
		}
		sub := submitted[ds]
		days = append(days, calendarDay{
			Date: ds, Weekday: int(d.Weekday()), IsWorkday: isWork,
			Holiday: holiday, Submitted: sub,
		})
		if isWork && ds < today {
			workdays++
			if sub {
				filledWorkdays++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"month":           month,
		"days":            days,
		"workdays":        workdays,
		"filled_workdays": filledWorkdays,
	})
}

// DaySummary handles GET /api/calendar/day?date=2026-03-02
func (h *CalendarHandler) DaySummary(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date required"})
		return
	}
	uid := c.GetInt("user_id")
	s, err := h.daily.GetSummary(c.Request.Context(), uid, date)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"date": date, "submitted": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"date": date, "submitted": true,
		"summary": s.Summary, "risk": s.Risk,
	})
}
