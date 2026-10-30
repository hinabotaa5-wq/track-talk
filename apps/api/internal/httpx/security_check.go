package httpx

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/mail"
)

// 見慣れない DB 接続の検知。いまは API も imports も同じ DB ユーザーなので、接続の名札
// （application_name）でしか出所を区別できない。名札のない・知らない接続があれば運営へ知らせる。
// 限界: 15分ごとの見回りなので短い接続は見逃すことがある。名札を真似られれば通る（対策は DB ユーザー分離）。

// 自前の名札。完全一致でだけ認める（"track-talk-api-2" のような偽装を通さない）。
var ownDBApplicationNames = map[string]bool{
	"track-talk-api":     true,
	"track-talk-import":  true,
	"track-talk-migrate": true,
}

// Supabase 内部の接続。pg_stat_activity での実測で増えたらここへ足す。
var supabaseDBApplicationNames = map[string]bool{
	"supavisor":         true,
	"postgrest":         true,
	"Supabase Auth":     true,
	"Supabase Realtime": true,
	"supabase_admin":    true,
}

func allowedDBApplicationName(name string) bool {
	return ownDBApplicationNames[name] || supabaseDBApplicationNames[name]
}

type dbConnection struct {
	PID             int32
	ApplicationName string
	BackendStart    time.Time
	State           string
}

// securityCheck は Cloud Scheduler から定期的に呼ばれる。秘密のヘッダーがないと受け付けない。
// コーチングの公開状態には寄せない（守りは公開前から要る）。
func (s *Server) securityCheck(w http.ResponseWriter, r *http.Request) {
	secret := s.cfg.SecurityCheckSecret
	got := r.Header.Get("X-Security-Check-Secret")
	if secret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	unknown, alerted, err := s.checkDBConnections(r.Context())
	if err != nil {
		slog.Error("security check", "err", err)
		Error(w, http.StatusInternalServerError, "internal_error", "処理できませんでした")
		return
	}
	JSON(w, http.StatusOK, map[string]any{"unknown_connections": unknown, "alerted": alerted})
}

// checkDBConnections は、同じ DB ユーザーの接続のうち名札が許可リストにないものを数え、
// 初めて見た接続だけ運営へメールする。メール本文には接続の名札・開始時刻・状態だけを入れ、
// クエリ本文や IP は入れない。
func (s *Server) checkDBConnections(ctx context.Context) (unknown int, alerted int, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pid, COALESCE(application_name, ''), backend_start, COALESCE(state, '')
		FROM pg_stat_activity
		WHERE usename = current_user AND pid <> pg_backend_pid() AND backend_type = 'client backend'`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	var found []dbConnection
	for rows.Next() {
		var c dbConnection
		if err := rows.Scan(&c.PID, &c.ApplicationName, &c.BackendStart, &c.State); err != nil {
			return 0, 0, err
		}
		if !allowedDBApplicationName(c.ApplicationName) {
			found = append(found, c)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	for _, c := range found {
		key := fmt.Sprintf("%d/%s", c.PID, c.BackendStart.Format(time.RFC3339Nano))
		if !s.markSecurityConnectionSeen(key) {
			continue
		}
		slog.Warn("unknown db connection", "application_name", c.ApplicationName, "backend_start", c.BackendStart, "state", c.State)
		if s.alertOps(ctx, "見慣れないデータベース接続がありました",
			fmt.Sprintf("接続の名札: %s\n接続開始: %s\n状態: %s\n\n心当たりがなければ、Supabase でデータベースのパスワードをリセットし、接続文字列を入れ替えてください。",
				displayAppName(c.ApplicationName), c.BackendStart.Format(time.RFC3339), c.State),
			"db-connection-"+key) {
			alerted++
		}
	}
	return len(found), alerted, nil
}

func displayAppName(name string) string {
	if name == "" {
		return "（なし）"
	}
	return name
}

// markSecurityConnectionSeen は、同じ接続を1回だけ通知するための記録。初めてなら true。
// プロセスが再起動すると記録は消えるので、その場合はもう1通届くことがある。
func (s *Server) markSecurityConnectionSeen(key string) bool {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()
	if s.securitySeen == nil {
		s.securitySeen = map[string]bool{}
	}
	if s.securitySeen[key] {
		return false
	}
	s.securitySeen[key] = true
	return true
}

// alertOps は運営への警告メール。宛先（OPS_ALERT_EMAIL）かメール設定がなければ送らず、
// ログにだけ残す（呼び出し側でログ済みの場合もある）。
func (s *Server) alertOps(ctx context.Context, subject, text, idempotencyKey string) bool {
	to := strings.TrimSpace(s.cfg.OpsAlertEmail)
	if to == "" || !s.mail.Configured() {
		return false
	}
	if err := s.mail.Send(ctx, mail.Message{To: to, Subject: "[TrackTalk] " + subject, Text: text}, idempotencyKey); err != nil {
		slog.Warn("ops alert mail", "err", err)
		return false
	}
	return true
}
