package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"smart-daily/internal/repository"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type ExportHandler struct{ dailyRepo *repository.DailyRepo }

func NewExportHandler(dailyRepo *repository.DailyRepo) *ExportHandler {
	return &ExportHandler{dailyRepo: dailyRepo}
}

func (h *ExportHandler) ExportDaily(c *gin.Context) {
	rows, err := h.dailyRepo.ListSummariesWithMembers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	f := excelize.NewFile()
	sheet := "日报"
	f.SetSheetName("Sheet1", sheet)

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#E5E7EB"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	wrapStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})

	f.SetColWidth(sheet, "A", "A", 14)
	f.SetColWidth(sheet, "B", "B", 50)
	f.SetColWidth(sheet, "C", "C", 30)

	r := 1
	lastDate := ""
	for _, item := range rows {
		dateStr := item.DailyDate
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			dateStr = t.Format("2006-01-02")
		} else if t, err := time.Parse("2006-01-02T15:04:05Z", dateStr); err == nil {
			dateStr = t.Format("2006-01-02")
		}
		if dateStr != lastDate {
			cell := fmt.Sprintf("A%d", r)
			f.MergeCell(sheet, cell, fmt.Sprintf("C%d", r))
			f.SetCellValue(sheet, cell, dateStr)
			f.SetCellStyle(sheet, cell, fmt.Sprintf("C%d", r), headerStyle)
			f.SetRowHeight(sheet, r, 28)
			r++
			lastDate = dateStr
		}
		f.SetCellValue(sheet, fmt.Sprintf("A%d", r), item.Name)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", r), item.Summary)
		if item.Risk != "" {
			f.SetCellValue(sheet, fmt.Sprintf("C%d", r), item.Risk)
		}
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", r), fmt.Sprintf("C%d", r), wrapStyle)
		r++
	}

	filename := fmt.Sprintf("日报导出_%s.xlsx", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(filename)))
	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
