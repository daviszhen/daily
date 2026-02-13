package main

import (
	"embed"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"smart-daily/internal/config"
	"smart-daily/internal/handler"
	applog "smart-daily/internal/log"
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
	applog.Init(cfg.Log)
	db, err := cfg.OpenGormDB()
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}

	raw, err := cfg.NewRawClient()
	if err != nil {
		slog.Warn("sdk client init failed", "err", err)
	}

	var catalogSync *service.CatalogSync
	if raw != nil {
		catalogSync = service.NewCatalogSync(raw)
		slog.Info("catalog sync enabled")
	}

	aiSvc := service.NewAIService(cfg.MOI.BaseURL, cfg.MOI.APIKey, raw)
	dailySvc := service.NewDailyService(db)
	authSvc := service.NewAuthService(db)

	chatH := handler.NewChatHandler(aiSvc, dailySvc, catalogSync)
	authH := handler.NewAuthHandler(authSvc)

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

	distFS, _ := fs.Sub(staticFS, "dist")
	r.NoRoute(gin.WrapH(http.FileServer(http.FS(distFS))))

	slog.Info("server starting", "addr", cfg.Addr())
	if err := r.Run(cfg.Addr()); err != nil {
		slog.Error("server failed", "err", err)
	}
}
