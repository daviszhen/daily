package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"net/http"
	"os"
	"smart-daily/internal/config"
	"smart-daily/internal/handler"
	"smart-daily/internal/logger"
	"smart-daily/internal/middleware"
	"smart-daily/internal/service"

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
	dailySvc := service.NewDailyService(db)
	authSvc := service.NewAuthService(db)

	// Sync members to Catalog at startup
	if catalogSync != nil && catalogSync.Ready() {
		go catalogSync.SyncAllMembers(db)
	}
	// Seed NL2SQL knowledge for Data Asking
	go aiSvc.SeedKnowledge(context.Background())

	chatH := handler.NewChatHandler(aiSvc, dailySvc, catalogSync)
	authH := handler.NewAuthHandler(authSvc)
	importH := handler.NewImportHandler(db, aiSvc, catalogSync)
	sessionSvc := service.NewSessionService(cfg.MOI.BaseURL, cfg.MOI.APIKey)
	sessionH := handler.NewSessionHandler(sessionSvc)

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

	distFS, _ := fs.Sub(staticFS, "dist")
	r.NoRoute(gin.WrapH(http.FileServer(http.FS(distFS))))

	logger.Info("server starting", "addr", cfg.Addr())
	if err := r.Run(cfg.Addr()); err != nil {
		logger.Error("server failed", "err", err)
	}
}
