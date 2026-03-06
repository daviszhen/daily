package main

import (
	"bufio"
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"smart-daily/internal/config"
	"strconv"
	"smart-daily/internal/handler"
	"smart-daily/internal/logger"
	"smart-daily/internal/middleware"
	"smart-daily/internal/model"
	"smart-daily/internal/repository"
	"smart-daily/internal/service"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var staticFS embed.FS

func main() {
	configFile := flag.String("config", "", "config file path (e.g. etc/config-dev.yaml)")
	flag.Parse()

	cfg := config.Load(*configFile)
	logger.Init(cfg.Log)
	db, err := cfg.OpenGormDB()
	if err != nil {
		logger.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	// Auto-create teams table if not exists
	db.Exec("CREATE TABLE IF NOT EXISTS teams (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(50) NOT NULL UNIQUE)")
	// Add team_id column to members (ignore error if already exists)
	db.Exec("ALTER TABLE members ADD COLUMN team_id INT DEFAULT 0")
	// Auto-create topic_activities table if not exists
	db.Exec("CREATE TABLE IF NOT EXISTS topic_activities (id INT AUTO_INCREMENT PRIMARY KEY, topic VARCHAR(100) NOT NULL, member_id INT NOT NULL, member_name VARCHAR(50) NOT NULL, daily_date DATE NOT NULL, content TEXT, entry_id INT DEFAULT 0, INDEX idx_topic (topic), INDEX idx_daily_date (daily_date))")
	db.Exec("CREATE TABLE IF NOT EXISTS topics (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100) NOT NULL UNIQUE, description TEXT DEFAULT '', status VARCHAR(20) DEFAULT 'active', created_at DATETIME DEFAULT NOW(), resolved_at DATETIME DEFAULT NULL)")
	db.Exec("CREATE TABLE IF NOT EXISTS feedback (id INT AUTO_INCREMENT PRIMARY KEY, member_id INT NOT NULL, member_name VARCHAR(50) NOT NULL, content TEXT NOT NULL, status VARCHAR(20) DEFAULT 'open', created_at DATETIME DEFAULT NOW())")

	raw, err := cfg.NewRawClient()
	if err != nil {
		logger.Warn("sdk client init failed", "err", err)
	}

	var catalogSync *service.CatalogSync
	if raw != nil {
		catalogSync = service.NewCatalogSync(raw, cfg.MOI.CatalogID, cfg.Database.Name, cfg.MOI.BaseURL, cfg.MOI.APIKey)
	}

	aiSvc := service.NewAIService(cfg.MOI.BaseURL, cfg.MOI.APIKey, cfg.MOI.Model, cfg.MOI.FastModel, cfg.Database.Name, raw)
	if catalogSync != nil && catalogSync.Ready() {
		aiSvc.SetCatalogDBID(catalogSync.DatabaseID())
	}
	// Repositories
	memberRepo := repository.NewMemberRepo(db)
	dailyRepo := repository.NewDailyRepo(db)
	topicRepo := repository.NewTopicRepo(db)

	// Services
	dailySvc := service.NewDailyService(dailyRepo)
	authSvc := service.NewAuthService(memberRepo)

	// Sync all data to Catalog at startup (idempotent, ConflictPolicyReplace)
	if catalogSync != nil && catalogSync.Ready() {
		go func() {
			catalogSync.SyncSchemaFromDB(db)
			catalogSync.TruncateAll()

			members, _ := memberRepo.ListActive(context.Background())
			catalogSync.SyncAllMembers(members)

			var teams []model.Team
			db.Find(&teams)
			if len(teams) > 0 {
				catalogSync.SyncAllTeams(teams)
			}

			var entries []model.DailyEntry
			db.Find(&entries)
			if len(entries) > 0 {
				catalogSync.SyncAllEntries(entries)
			}

			var summaries []model.DailySummary
			db.Find(&summaries)
			if len(summaries) > 0 {
				catalogSync.SyncAllSummaries(summaries)
			}

			var topics []model.Topic
			db.Find(&topics)
			if len(topics) > 0 {
				catalogSync.SyncAllTopics(topics)
			}

			var activities []model.TopicActivity
			db.Find(&activities)
			if len(activities) > 0 {
				catalogSync.SyncAllTopicActivities(activities)
			}
		}()
	}
	// Seed NL2SQL knowledge for Data Asking
	go aiSvc.SeedKnowledge(context.Background())

	// Auto-extract topics for historical entries (one-time, on first deploy)
	go func() {
		var topicCount int64
		db.Model(&model.TopicActivity{}).Count(&topicCount)
		var entries []model.DailyEntry
		db.Find(&entries)
		if len(entries) == 0 {
			return
		}
		if topicCount > int64(len(entries)/2) {
			return
		}
		if topicCount > 0 {
			db.Where("1=1").Delete(&model.TopicActivity{})
		}
		start := time.Now()
		logger.Info("auto-extracting topics (batch mode)", "entries", len(entries))
		members, _ := memberRepo.ListActive(context.Background())
		nameMap := make(map[int]string)
		for _, m := range members {
			nameMap[m.ID] = m.Name
		}
		existingTopics, _ := topicRepo.ListDistinctTopics(context.Background())

		// Group entries into batches of 20
		const batchSize = 20
		type batch struct {
			contents map[int]string
			entries  map[int]model.DailyEntry
		}
		var batches []batch
		cur := batch{contents: map[int]string{}, entries: map[int]model.DailyEntry{}}
		for _, e := range entries {
			cur.contents[e.ID] = e.Summary
			cur.entries[e.ID] = e
			if len(cur.contents) >= batchSize {
				batches = append(batches, cur)
				cur = batch{contents: map[int]string{}, entries: map[int]model.DailyEntry{}}
			}
		}
		if len(cur.contents) > 0 {
			batches = append(batches, cur)
		}

		sem := make(chan struct{}, 10)
		var mu sync.Mutex
		var allItems []model.TopicActivity
		var wg sync.WaitGroup
		var done int64
		total := int64(len(batches))

		for _, b := range batches {
			wg.Add(1)
			sem <- struct{}{}
			go func(b batch) {
				defer wg.Done()
				defer func() { <-sem }()
				result, err := aiSvc.ExtractTopicsBatch(context.Background(), b.contents, existingTopics)
				n := atomic.AddInt64(&done, 1)
				if n%20 == 0 || n == total {
					logger.Info("topic extract progress", "batches", n, "total", total, "elapsed", time.Since(start).Round(time.Second))
				}
				if err != nil {
					logger.Warn("topic batch extract failed", "err", err)
					return
				}
				mu.Lock()
				for entryID, topics := range result {
					e, ok := b.entries[entryID]
					if !ok {
						continue
					}
					date := e.DailyDate
					if len(date) > 10 {
						date = date[:10]
					}
					for _, t := range topics {
						allItems = append(allItems, model.TopicActivity{
							Topic: t, MemberID: e.MemberID, MemberName: nameMap[e.MemberID],
							DailyDate: date, Content: e.Summary, EntryID: e.ID,
						})
					}
				}
				mu.Unlock()
			}(b)
		}
		wg.Wait()
		if len(allItems) > 0 {
			topicSet := map[string]bool{}
			for _, item := range allItems {
				topicSet[item.Topic] = true
			}
			var topicNames []string
			for t := range topicSet {
				topicNames = append(topicNames, t)
			}
			topicRepo.EnsureTopics(context.Background(), topicNames)
			const chunk = 200
			for i := 0; i < len(allItems); i += chunk {
				end := i + chunk
				if end > len(allItems) {
					end = len(allItems)
				}
				if err := topicRepo.BatchCreate(context.Background(), allItems[i:end]); err != nil {
					logger.Error("topic batch create failed", "offset", i, "err", err)
				}
			}
			logger.Info("auto-extract topics done", "entries", len(entries), "activities", len(allItems), "topics", len(topicSet), "elapsed", time.Since(start).Round(time.Second))
		}
	}()

	importSvc := service.NewImportService(aiSvc, memberRepo, dailyRepo, topicRepo, catalogSync)
	chatH := handler.NewChatHandler(aiSvc, dailySvc, catalogSync, topicRepo, memberRepo)
	authH := handler.NewAuthHandler(authSvc)
	importH := handler.NewImportHandler(importSvc)
	sessionSvc := service.NewSessionService(cfg.MOI.BaseURL, cfg.MOI.APIKey)
	sessionH := handler.NewSessionHandler(sessionSvc)
	memberH := handler.NewMemberHandler(memberRepo)
	exportH := handler.NewExportHandler(dailyRepo)
	feedH := handler.NewFeedHandler(topicRepo)
	holidaySvc := service.NewHolidayService()
	calendarH := handler.NewCalendarHandler(dailyRepo, holidaySvc)

	chatH.SetSessionService(sessionSvc)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	r.POST("/api/login", authH.Login)
	api := r.Group("/api", middleware.JWTAuth())
	api.POST("/chat", chatH.Chat)
	api.POST("/chat/stream", chatH.ChatStream)
	api.GET("/files/:name", chatH.DownloadFile)
	api.POST("/sessions", sessionH.Create)
	api.GET("/sessions", sessionH.List)
	api.DELETE("/sessions/:id", sessionH.Delete)
	api.GET("/sessions/:id/messages", sessionH.Messages)
	api.POST("/import/preview", importH.Preview)
	api.POST("/import/confirm", importH.Confirm)
	api.GET("/members", memberH.List)
	api.GET("/teams", memberH.ListTeams)
	// Admin-only: member/team management
	admin := api.Group("", middleware.AdminOnly())
	admin.PUT("/members/:id", memberH.Update)
	admin.DELETE("/members/:id", memberH.Delete)
	admin.POST("/teams", memberH.CreateTeam)
	// Logs (admin-only)
	admin.GET("/logs", logsHandler(cfg.Log.File))
	admin.GET("/logs/stream", logsStreamHandler(cfg.Log.File))
	// Feed & Insights
	api.GET("/feed/by-member", feedH.FeedByMember)
	api.GET("/feed/by-topic", feedH.FeedByTopic)
	api.GET("/insights", feedH.Insights)
	// Topic management
	api.GET("/topics/all", feedH.ListTopics)
	api.PUT("/topics/:id", feedH.UpdateTopic)
	api.PUT("/topics/:id/resolve", feedH.ResolveTopic)
	api.PUT("/topics/:id/reopen", feedH.ReopenTopic)
	api.POST("/topics/merge", feedH.MergeTopic)
	api.GET("/export/daily", exportH.ExportDaily)
	api.GET("/calendar", calendarH.Calendar)
	api.GET("/calendar/day", calendarH.DaySummary)
	// Feedback
	fbH := handler.NewFeedbackHandler(db)
	api.POST("/feedback", fbH.Submit)
	api.GET("/feedback", fbH.List)
	admin.PUT("/feedback/:id/close", fbH.Close)
	admin.DELETE("/feedback/:id", fbH.Delete)

	distFS, _ := fs.Sub(staticFS, "dist")
	r.NoRoute(gin.WrapH(http.FileServer(http.FS(distFS))))

	logger.Info("server starting", "addr", cfg.Addr())
	if err := r.Run(cfg.Addr()); err != nil {
		logger.Error("server failed", "err", err)
	}
}

// logsHandler returns last N lines of the log file. ?lines=200&download=true
func logsHandler(logFile string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if logFile == "" {
			c.JSON(400, gin.H{"error": "log file not configured"})
			return
		}
		f, err := os.Open(logFile)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer f.Close()

		if c.Query("download") == "true" {
			c.Header("Content-Disposition", "attachment; filename=backend.log")
			c.Header("Content-Type", "text/plain")
			io.Copy(c.Writer, f)
			return
		}

		n := 100
		if v, err := strconv.Atoi(c.Query("lines")); err == nil && v > 0 {
			n = v
		}
		// read all lines, keep last N
		var lines []string
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
		c.Header("Content-Type", "text/plain")
		for _, l := range lines {
			fmt.Fprintln(c.Writer, l)
		}
	}
}

// logsStreamHandler streams log file via SSE (tail -f style)
func logsStreamHandler(logFile string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if logFile == "" {
			c.JSON(400, gin.H{"error": "log file not configured"})
			return
		}
		f, err := os.Open(logFile)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer f.Close()

		// seek to last 8KB for initial context
		if info, err := f.Stat(); err == nil && info.Size() > 8192 {
			f.Seek(-8192, io.SeekEnd)
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		reader := bufio.NewReader(f)
		ctx := c.Request.Context()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				fmt.Fprintf(c.Writer, "data: %s\n\n", line)
				c.Writer.Flush()
			}
			if err != nil {
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}
