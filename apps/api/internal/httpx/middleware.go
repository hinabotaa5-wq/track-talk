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

func OptionalAuth(v *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw := bearer(r); raw != "" {
				identity, err := v.ParseIdentity(raw)
				if err == nil {
					r = r.WithContext(withUser(r.Context(), identity))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAuth(v *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearer(r)
			if raw == "" {
				Error(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
				return
			}
			identity, err := v.ParseIdentity(raw)
			if err != nil {
				Error(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
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
	mu        sync.Mutex
	writes    map[string][]time.Time
	reports   map[string][]time.Time
	inquiries map[string][]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
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
		ip := clientIP(r)
		now := time.Now()
		l.mu.Lock()
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
		l.writes[ip] = prune(l.writes[ip], now.Add(-time.Minute))
		if len(l.writes[ip]) >= 30 {
			l.mu.Unlock()
			Error(w, http.StatusTooManyRequests, "rate_limited", "リクエストが多すぎます")
			return
		}
		l.writes[ip] = append(l.writes[ip], now)
		l.mu.Unlock()
		next.ServeHTTP(w, r)
	})
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
