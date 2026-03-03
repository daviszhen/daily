package main

import (
	"context"
	"flag"
	"log"

	"smart-daily/internal/config"
	"smart-daily/internal/logger"

	sdk "github.com/matrixorigin/moi-go-sdk"
)

func main() {
	configFile := flag.String("config", "etc/config-dev.yaml", "config file")
	flag.Parse()

	logger.Init(config.LogConfig{Level: "info", Console: true})

	cfg := config.Load(*configFile)
	client, err := cfg.NewRawClient()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	catalogID := sdk.CatalogID(cfg.MOI.CatalogID)
	if catalogID == 0 {
		catalogID = 1
	}

	// Step 1: catalog database + tables
	if _, err := initCatalog(ctx, client, catalogID, cfg.Database.Name); err != nil {
		log.Fatal("catalog init failed:", err)
	}

	// Step 2: NL2SQL knowledge
	if err := initKnowledge(ctx, client); err != nil {
		log.Fatal("knowledge init failed:", err)
	}

	logger.Info("=== all done ===")
}
