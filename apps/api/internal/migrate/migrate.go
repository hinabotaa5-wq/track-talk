package migrate

import (
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/hinata/track-talk/apps/api/migrations"
)

// withAppName は、接続の名札（application_name）が入っていない接続文字列に移行用の名札を付ける。
// 接続文字列そのもの（秘密）は変えず、クエリに1項目を足すだけ。
func withAppName(databaseURL string) string {
	u, err := url.Parse(databaseURL)
	if err != nil || u.Scheme == "" {
		return databaseURL
	}
	q := u.Query()
	if q.Get("application_name") == "" {
		q.Set("application_name", "track-talk-migrate")
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func Up(databaseURL string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, withAppName(databaseURL))
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
