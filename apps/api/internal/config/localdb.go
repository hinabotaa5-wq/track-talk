package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

// RequireLocalDatabase は、DATABASE_URL がこのマシン上の DB でなければエラーにする。
// 開発用のコマンド(api・migrate・テスト)が誤って本番 DB に当たるのを防ぐ。
// 本番に意図して流すときだけ ALLOW_REMOTE_DB=1 を付ける(本番イメージは Dockerfile で設定済み)。
func RequireLocalDatabase(dsn string) error {
	if os.Getenv("ALLOW_REMOTE_DB") == "1" {
		return nil
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("DATABASE_URL を解釈できません: %w", err)
	}
	// ?host= / ?hostaddr= は URL のホストより優先されるので、そちらも検査する。
	hosts := []string{u.Hostname()}
	hosts = append(hosts, u.Query()["host"]...)
	hosts = append(hosts, u.Query()["hostaddr"]...)
	for _, h := range hosts {
		if !isLocalHost(h) {
			return fmt.Errorf("DATABASE_URL の接続先 %q はローカルではありません。開発では docker の PostgreSQL を使ってください(本番に流すなら ALLOW_REMOTE_DB=1)", h)
		}
	}
	return nil
}

func isLocalHost(h string) bool {
	switch h {
	case "localhost", "127.0.0.1", "::1", "postgres":
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
