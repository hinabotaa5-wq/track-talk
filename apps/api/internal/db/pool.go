package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 接続の名札（pg_stat_activity.application_name）。見慣れない接続の検知（/internal/security/check）が使う。
const (
	AppNameAPI     = "track-talk-api"
	AppNameImport  = "track-talk-import"
	AppNameMigrate = "track-talk-migrate"
)

// NewPool は、接続に application_name の名札を付けてプールを作る。接続文字列そのものは変えない。
func NewPool(ctx context.Context, databaseURL, appName string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = appName
	return pgxpool.NewWithConfig(ctx, cfg)
}
