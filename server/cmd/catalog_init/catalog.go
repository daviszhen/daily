package main

import (
	"context"
	"fmt"
	"smart-daily/internal/logger"
	"strings"

	sdk "github.com/matrixorigin/moi-go-sdk"
)

func initCatalog(ctx context.Context, client *sdk.RawClient, catalogID sdk.CatalogID, dbName string) (sdk.DatabaseID, error) {
	// 1. Create database
	dbResp, err := client.CreateDatabase(ctx, &sdk.DatabaseCreateRequest{
		CatalogID:    catalogID,
		DatabaseName: dbName,
		Comment:      "MOI 智能日报",
	})
	if err != nil {
		if isDuplicate(err) {
			logger.Info("catalog: database already exists, discovering ID", "name", dbName)
			return discoverDatabaseID(ctx, client, catalogID, dbName)
		}
		return 0, fmt.Errorf("create database: %w", err)
	}
	logger.Info("catalog: database created", "id", dbResp.DatabaseID)

	// 2. Create tables
	tables := []struct {
		name    string
		columns []sdk.Column
	}{
		{"members", []sdk.Column{
			{Name: "id", Type: "INT", IsPk: true, Comment: "主键"},
			{Name: "username", Type: "VARCHAR(50)", Comment: "登录用户名"},
			{Name: "password", Type: "VARCHAR(255)", Comment: "密码哈希"},
			{Name: "name", Type: "VARCHAR(50)", Comment: "团队成员真实姓名"},
			{Name: "avatar", Type: "VARCHAR(255)", Comment: "头像URL"},
			{Name: "role", Type: "VARCHAR(50)", Comment: "职位角色，如开发工程师、测试"},
			{Name: "team", Type: "VARCHAR(50)", Comment: "所属团队"},
		}},
		{"daily_entries", []sdk.Column{
			{Name: "id", Type: "INT", IsPk: true, Comment: "主键"},
			{Name: "member_id", Type: "INT", Comment: "关联members.id"},
			{Name: "daily_date", Type: "DATE", Comment: "日报所属日期"},
			{Name: "content", Type: "TEXT", Comment: "用户提交的原始工作内容"},
			{Name: "summary", Type: "TEXT", Comment: "AI生成的工作摘要"},
			{Name: "source", Type: "VARCHAR(20)", Comment: "提交来源：chat"},
			{Name: "created_at", Type: "DATETIME", Comment: "记录创建时间"},
		}},
		{"daily_summaries", []sdk.Column{
			{Name: "id", Type: "INT", IsPk: true, Comment: "主键"},
			{Name: "member_id", Type: "INT", Comment: "关联members.id"},
			{Name: "daily_date", Type: "DATE", Comment: "日报所属日期"},
			{Name: "summary", Type: "TEXT", Comment: "AI生成的工作摘要"},
			{Name: "status", Type: "TEXT", Comment: "工作状态"},
			{Name: "risk", Type: "TEXT", Comment: "AI检测到的风险项"},
			{Name: "blocker", Type: "TEXT", Comment: "阻塞问题"},
		}},
	}

	for _, t := range tables {
		resp, err := client.CreateTable(ctx, &sdk.TableCreateRequest{
			DatabaseID: dbResp.DatabaseID,
			Name:       t.name,
			Columns:    t.columns,
			Comment:    t.name,
		})
		if err != nil {
			if isDuplicate(err) {
				logger.Info("catalog: table already exists, skipping", "name", t.name)
				continue
			}
			return 0, fmt.Errorf("create table %s: %w", t.name, err)
		}
		logger.Info("catalog: table created", "name", t.name, "id", resp.TableID)
	}

	return dbResp.DatabaseID, nil
}

func discoverDatabaseID(ctx context.Context, client *sdk.RawClient, catalogID sdk.CatalogID, dbName string) (sdk.DatabaseID, error) {
	resp, err := client.ListDatabases(ctx, &sdk.DatabaseListRequest{CatalogID: catalogID})
	if err != nil {
		return 0, fmt.Errorf("list databases: %w", err)
	}
	for _, db := range resp.List {
		if db.DatabaseName == dbName {
			logger.Info("catalog: database discovered", "id", db.DatabaseID)
			return db.DatabaseID, nil
		}
	}
	return 0, fmt.Errorf("database %s not found in catalog %d", dbName, catalogID)
}

func isDuplicate(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "duplicate") || strings.Contains(s, "already exist") || strings.Contains(s, "exists") || strings.Contains(s, "conflict")
}
