package config

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	gomysql "github.com/go-sql-driver/mysql"
	sdk "github.com/matrixorigin/moi-go-sdk"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Log      LogConfig      `yaml:"log"`
	MOI      MOIConfig      `yaml:"moi"`
	Database DatabaseConfig `yaml:"database"`
}

type LogConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	Console    bool   `yaml:"console"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type MOIConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

func Load(configFile string) *Config {
	c := &Config{
		Server:   ServerConfig{Port: 9871},
		MOI:      MOIConfig{BaseURL: "https://freetier-01.cn-hangzhou.cluster.cn-dev.matrixone.tech"},
		Log:      LogConfig{Level: "info", Console: true, MaxSizeMB: 100, MaxBackups: 3, MaxAgeDays: 30},
		Database: DatabaseConfig{Port: 6001, Name: "smart_daily"},
	}

	paths := []string{"etc/config-dev.yaml", "/etc/smart-daily/config.yaml"}
	if configFile != "" {
		paths = []string{configFile}
	}
	for _, path := range paths {
		if data, err := os.ReadFile(path); err == nil {
			yaml.Unmarshal(data, c)
			break
		}
	}

	envOverride(&c.MOI.BaseURL, "MOI_BASE_URL")
	envOverride(&c.MOI.APIKey, "MOI_API_KEY")
	envOverride(&c.Database.Host, "MO_HOST")
	envOverride(&c.Database.User, "MO_USER")
	envOverride(&c.Database.Password, "MO_PASS")
	envOverride(&c.Database.Name, "MO_DB")
	envOverride(&c.Log.Level, "LOG_LEVEL")
	envOverride(&c.Log.File, "LOG_FILE")
	envOverrideInt(&c.Server.Port, "PORT")
	envOverrideInt(&c.Database.Port, "MO_PORT")

	return c
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.Server.Port)
}

func (c *Config) OpenGormDB() (*gorm.DB, error) {
	cfg := gomysql.NewConfig()
	cfg.User = c.Database.User
	cfg.Passwd = c.Database.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", c.Database.Host, c.Database.Port)
	cfg.DBName = c.Database.Name
	cfg.ParseTime = true

	connector, err := gomysql.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("create connector: %w", err)
	}
	sqlDB := sql.OpenDB(connector)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

func (c *Config) NewRawClient() (*sdk.RawClient, error) {
	return sdk.NewRawClient(c.MOI.BaseURL, c.MOI.APIKey)
}

func envOverride(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

func envOverrideInt(dst *int, key string) {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			*dst = n
		}
	}
}
