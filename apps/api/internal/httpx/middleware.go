package httpx

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/auth"
)

type ctxKey int

const identityKey ctxKey = iota

func UserID(ctx context.Context) (uuid.UUID, bool) {
	identity, ok := ctx.Value(identityKey).(auth.Identity)
	return identity.UserID, ok && identity.UserID != uuid.Nil
}

func userIdentity(ctx context.Context) (auth.Identity, bool) {
	identity, ok := ctx.Value(identityKey).(auth.Identity)
	return identity, ok && identity.UserID != uuid.Nil
}

func withUser(ctx context.Context, identity auth.Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "recover", rec, "path", logPath(r.URL.Path))
				Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r)
		uid, _ := UserID(r.Context())
		slog.Info("http",
			"request_id", id,
			"method", r.Method,
			"path", logPath(r.URL.Path),
			"status", ww.status,
			"ms", time.Since(start).Milliseconds(),
			"user_id", uid.String(),
		)
	})
}

// logPath は、ログに残すパス。保護者のリンクのトークンは、知っていれば保護者として
// 同意やメッセージができてしまうので、ログには残さない。
func logPath(path string) string {
	const prefix = "/v1/guardian/"
	rest, ok := strings.CutPrefix(path, prefix)
	if !ok {
		return path
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return prefix + ":token" + rest[i:]
	}
	return prefix + ":token"
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// mfaPending は、二段階認証を設定済みなのに、いまのログインでコードを入れていない(aal2 でない)ときに true。
func mfaPending(ctx context.Context, mfa *MFAEnrollment, identity auth.Identity) (bool, error) {
	if identity.AAL == "aal2" {
		return false, nil
	}
	return mfa.Enrolled(ctx, identity.UserID)
}

// OptionalAuth は、ログインしていれば利用者を入れる。二段階認証のコードを入れる前のログインは、ログインしていないものとして扱う。
func OptionalAuth(v *auth.Verifier, mfa *MFAEnrollment) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw := bearer(r); raw != "" {
				identity, err := v.ParseIdentity(raw)
				if err == nil {
					if pending, err := mfaPending(r.Context(), mfa, identity); err != nil {
						slog.Error("mfa check", "err", err)
					} else if !pending {
						r = r.WithContext(withUser(r.Context(), identity))
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth は、ログインしていなければ 401 を返す。
// 二段階認証を設定済みの人が、コードを入れる前のログインで呼んだら 403(mfa_required)を返す。
func RequireAuth(v *auth.Verifier, mfa *MFAEnrollment) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearer(r)
			if raw == "" {
				Error(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
				return
			}
			identity, err := v.ParseIdentity(raw)
			if err != nil {
				// 鍵の入れ替えなどで全員が弾かれたときに理由を追えるよう、失敗の理由だけ残す(トークンは残さない)。
				slog.Warn("auth rejected", "err", err, "path", logPath(r.URL.Path))
				Error(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
				return
			}
			pending, err := mfaPending(r.Context(), mfa, identity)
			if err != nil {
				slog.Error("mfa check", "err", err)
				Error(w, http.StatusServiceUnavailable, "unavailable", "いまは確認できません。しばらくしてからもう一度試してください")
				return
			}
			if pending {
				Error(w, http.StatusForbidden, "mfa_required", "二段階認証のコードを入れてください")
				return
			}
			next.ServeHTTP(w, r.WithContext(withUser(r.Context(), identity)))
		})
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return ""
	}
	return strings.TrimSpace(h[7:])
}

type RateLimiter struct {
	// auth は、書き込みを人ごとに数えるためにトークンを読む。nil なら全部 IP で数える。
	auth      *auth.Verifier
	mu        sync.Mutex
	writes    map[string][]time.Time
	reports   map[string][]time.Time
	inquiries map[string][]time.Time
	// lastSweep は、来なくなった IP の行を最後にまとめて消した時刻。
	lastSweep time.Time
}

func NewRateLimiter(v *auth.Verifier) *RateLimiter {
	return &RateLimiter{
		auth:      v,
		writes:    map[string][]time.Time{},
		reports:   map[string][]time.Time{},
		inquiries: map[string][]time.Time{},
	}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Stripe の Webhook は Stripe のサーバーからまとめて届くので数えない（署名で確かめる）。
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || r.URL.Path == "/webhooks/stripe" {
			next.ServeHTTP(w, r)
			return
		}
		ip := rateKey(clientIP(r))
		now := time.Now()
		l.mu.Lock()
		l.sweep(now)
		if strings.HasPrefix(r.URL.Path, "/v1/reports") {
			l.reports[ip] = prune(l.reports[ip], now.Add(-time.Hour))
			if len(l.reports[ip]) >= 10 {
				l.mu.Unlock()
				Error(w, http.StatusTooManyRequests, "rate_limited", "通報の回数制限に達しました")
				return
			}
			l.reports[ip] = append(l.reports[ip], now)
		}
		if r.URL.Path == "/v1/me/inquiries" {
			l.inquiries[ip] = prune(l.inquiries[ip], now.Add(-time.Hour))
			if len(l.inquiries[ip]) >= 10 {
				l.mu.Unlock()
				Error(w, http.StatusTooManyRequests, "rate_limited", "お問い合わせの回数制限に達しました")
				return
			}
			l.inquiries[ip] = append(l.inquiries[ip], now)
		}
		// 同じ Wi-Fi や携帯回線の部員は IP を分け合うので、ログイン中は人ごとに数える。
		key := l.writeKey(r, ip)
		l.writes[key] = prune(l.writes[key], now.Add(-time.Minute))
		if len(l.writes[key]) >= 30 {
			l.mu.Unlock()
			Error(w, http.StatusTooManyRequests, "rate_limited", "リクエストが多すぎます")
			return
		}
		l.writes[key] = append(l.writes[key], now)
		l.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

// writeKey は、書き込みの回数を数える単位を返す。正しいトークンがあれば利用者、なければ IP。
func (l *RateLimiter) writeKey(r *http.Request, ip string) string {
	if l.auth == nil {
		return ip
	}
	raw := bearer(r)
	if raw == "" {
		return ip
	}
	identity, err := l.auth.ParseIdentity(raw)
	if err != nil {
		return ip
	}
	return "user:" + identity.UserID.String()
}

// sweep は10分ごとに、窓の外の記録しか残っていない IP の行を消す。消さないと、送信元を変えながら
// 送られるだけ map が増え続ける。l.mu を持って呼ぶ。
func (l *RateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < 10*time.Minute {
		return
	}
	l.lastSweep = now
	for m, window := range map[*map[string][]time.Time]time.Duration{&l.writes: time.Minute, &l.reports: time.Hour, &l.inquiries: time.Hour} {
		for ip, times := range *m {
			if times = prune(times, now.Add(-window)); len(times) == 0 {
				delete(*m, ip)
			} else {
				(*m)[ip] = times
			}
		}
	}
}

func prune(in []time.Time, cutoff time.Time) []time.Time {
	out := in[:0]
	for _, t := range in {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

func clientIP(r *http.Request) string {
	host := requestHost(r)
	if !trustedProxy(host) {
		return host
	}
	if ip := forwardedClient(r.Header.Get("X-Forwarded-For")); ip != "" {
		return ip
	}
	return host
}

// rateKey は、回数制限の数え方の単位を返す。IPv6 は1人が /64 を丸ごと持つので、/64 でまとめて数える。
func rateKey(host string) string {
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() != nil {
		return host
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

func requestHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trustedProxy(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}

func forwardedClient(header string) string {
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip == nil {
			continue
		}
		return ip.String()
	}
	return ""
}
