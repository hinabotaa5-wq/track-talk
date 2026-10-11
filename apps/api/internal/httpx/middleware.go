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

const (
	userIDKey ctxKey = iota
	requestIDKey
)

func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(requestIDKey).(string)
	return s
}

func withUser(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "recover", rec, "path", r.URL.Path)
				Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r.WithContext(ctx))
		uid, _ := UserID(ctx)
		slog.Info("http",
			"request_id", id,
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"ms", time.Since(start).Milliseconds(),
			"user_id", uid.String(),
		)
	})
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
				id, err := v.Parse(raw)
				if err == nil {
					r = r.WithContext(withUser(r.Context(), id))
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
			id, err := v.Parse(raw)
			if err != nil {
				Error(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
				return
			}
			next.ServeHTTP(w, r.WithContext(withUser(r.Context(), id)))
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
	mu      sync.Mutex
	writes  map[string][]time.Time
	reports map[string][]time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		writes:  map[string][]time.Time{},
		reports: map[string][]time.Time{},
	}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
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
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
