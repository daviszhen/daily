package main

import (
	"context"
	"smart-daily/internal/logger"
	"smart-daily/internal/service"

	sdk "github.com/matrixorigin/moi-go-sdk"
)

func initKnowledge(ctx context.Context, client *sdk.RawClient) error {
	// Collect existing keys to avoid duplicates
	existing := map[string]bool{}
	list, err := client.ListKnowledge(ctx, &sdk.NL2SQLKnowledgeListRequest{PageNumber: 1, PageSize: 100})
	if err == nil {
		for _, k := range list.List {
			existing[k.Key] = true
		}
	}

	created := 0
	for _, k := range service.KnowledgeEntries() {
		if existing[k.Key] {
			continue
		}
		resp, err := client.CreateKnowledge(ctx, &k)
		if err != nil {
			return err
		}
		logger.Info("knowledge: created", "type", k.Type, "key", k.Key, "id", resp.ID)
		created++
	}
	logger.Info("knowledge: done", "created", created, "existing", len(existing))
	return nil
}
