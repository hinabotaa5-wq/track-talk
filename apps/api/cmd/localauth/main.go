// localauth は、ローカル確認専用の認証サービス。
// 本番の Supabase Auth の代わりに Docker 内で動き、Web が使う Supabase Auth 互換の
// API(/auth/v1/...)を返す。パスワードは bcrypt でハッシュ化して専用 DB に保存し、
// アクセストークンは本番と同じ HS256 の JWT(SUPABASE_JWT_SECRET と同じ鍵)で署名する。
// そのため API 側は本番と同じ検証コードのまま、ログイン済みとして扱える。
// 本番では使わない(cmd/api とは別のバイナリで、docker-compose の local スタックだけが起動する)。
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type user struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type store interface {
	CreateUser(ctx context.Context, email, hash string, id uuid.UUID) (user, error)
	UserByEmail(ctx context.Context, email string) (user, error)
	UserByID(ctx context.Context, id uuid.UUID) (user, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error
	UpdateEmail(ctx context.Context, id uuid.UUID, email string) error
	DeleteUser(ctx context.Context, id uuid.UUID) error
	SaveRefresh(ctx context.Context, userID uuid.UUID, tokenHash string, expires time.Time) error
	ConsumeRefresh(ctx context.Context, tokenHash string) (uuid.UUID, error)
	SaveRecovery(ctx context.Context, userID uuid.UUID, tokenHash string, expires time.Time) error
	ConsumeRecovery(ctx context.Context, tokenHash string) (uuid.UUID, error)
}

type pgStore struct{ pool *pgxpool.Pool }

var errNotFound = errors.New("not found")
var errDuplicate = errors.New("duplicate")

func (s pgStore) CreateUser(ctx context.Context, email, hash string, id uuid.UUID) (user, error) {
	var u user
	err := s.pool.QueryRow(ctx,
		`INSERT INTO auth_users (id, email, password_hash) VALUES ($1,$2,$3)
		 RETURNING id, email, password_hash, created_at`, id, email, hash).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return user{}, errDuplicate
		}
		return user{}, err
	}
	return u, nil
}

func scanUser(row pgx.Row) (user, error) {
	var u user
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return user{}, errNotFound
	}
	return u, err
}

func (s pgStore) UserByEmail(ctx context.Context, email string) (user, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at FROM auth_users WHERE email=$1`, email))
}

func (s pgStore) UserByID(ctx context.Context, id uuid.UUID) (user, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, created_at FROM auth_users WHERE id=$1`, id))
}

func (s pgStore) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE auth_users SET password_hash=$2 WHERE id=$1`, id, hash)
	return err
}

func (s pgStore) UpdateEmail(ctx context.Context, id uuid.UUID, email string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE auth_users SET email=$2 WHERE id=$1`, id, email)
	if err != nil && strings.Contains(err.Error(), "unique") {
		return errDuplicate
	}
	if err == nil && tag.RowsAffected() == 0 {
		return errNotFound
	}
	return err
}

func (s pgStore) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_users WHERE id=$1`, id)
	return err
}

func (s pgStore) SaveRefresh(ctx context.Context, userID uuid.UUID, tokenHash string, expires time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO auth_refresh_tokens (token_hash, user_id, expires_at) VALUES ($1,$2,$3)`, tokenHash, userID, expires)
	return err
}

func (s pgStore) ConsumeRefresh(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`DELETE FROM auth_refresh_tokens WHERE token_hash=$1 AND expires_at > now() RETURNING user_id`, tokenHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, errNotFound
	}
	return id, err
}

func (s pgStore) SaveRecovery(ctx context.Context, userID uuid.UUID, tokenHash string, expires time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO auth_recovery_tokens (token_hash, user_id, expires_at) VALUES ($1,$2,$3)`, tokenHash, userID, expires)
	return err
}

func (s pgStore) ConsumeRecovery(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`DELETE FROM auth_recovery_tokens WHERE token_hash=$1 AND expires_at > now() RETURNING user_id`, tokenHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, errNotFound
	}
	return id, err
}

type server struct {
	store       store
	secret      []byte
	externalURL string // ブラウザから見たこのサービスの URL(末尾 /auth/v1 まで含む)
	serviceKey  string
	seedEmail   string
	seedPass    string
	seedID      uuid.UUID
}

func main() {
	addr := env("LOCALAUTH_ADDR", ":9999")
	dsn := os.Getenv("AUTH_DATABASE_URL")
	if dsn == "" {
		slog.Error("AUTH_DATABASE_URL is required")
		os.Exit(1)
	}
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		slog.Error("SUPABASE_JWT_SECRET is required")
		os.Exit(1)
	}
	ctx := context.Background()
	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 30; i++ {
		pool, err = pgxpool.New(ctx, dsn)
		if err == nil {
			err = pool.Ping(ctx)
		}
		if err == nil {
			break
		}
		slog.Info("waiting for auth db", "err", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		slog.Error("auth db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS auth_users (
			id uuid PRIMARY KEY,
			email text NOT NULL UNIQUE,
			password_hash text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS auth_refresh_tokens (
			token_hash text PRIMARY KEY,
			user_id uuid NOT NULL REFERENCES auth_users (id) ON DELETE CASCADE,
			expires_at timestamptz NOT NULL
		);
		CREATE TABLE IF NOT EXISTS auth_recovery_tokens (
			token_hash text PRIMARY KEY,
			user_id uuid NOT NULL REFERENCES auth_users (id) ON DELETE CASCADE,
			expires_at timestamptz NOT NULL
		);`); err != nil {
		slog.Error("auth migrate", "err", err)
		os.Exit(1)
	}
	s := &server{
		store:       pgStore{pool},
		secret:      []byte(secret),
		externalURL: strings.TrimRight(env("LOCALAUTH_EXTERNAL_URL", "http://localhost:9998/auth/v1"), "/"),
		serviceKey:  os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		seedEmail:   strings.ToLower(strings.TrimSpace(os.Getenv("LOCALAUTH_SEED_EMAIL"))),
		seedPass:    os.Getenv("LOCALAUTH_SEED_PASSWORD"),
	}
	if v := os.Getenv("LOCALAUTH_SEED_USER_ID"); v != "" {
		s.seedID, _ = uuid.Parse(v)
	}
	if err := s.seed(ctx); err != nil {
		slog.Error("seed demo user", "err", err)
		os.Exit(1)
	}
	slog.Info("localauth listen", "addr", addr)
	if err := http.ListenAndServe(addr, s.routes()); err != nil {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (s *server) seed(ctx context.Context) error {
	if s.seedEmail == "" || s.seedPass == "" {
		return nil
	}
	if _, err := s.store.UserByEmail(ctx, s.seedEmail); err == nil {
		return nil
	} else if !errors.Is(err, errNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.seedPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	id := s.seedID
	if id == uuid.Nil {
		id = uuid.New()
	}
	_, err = s.store.CreateUser(ctx, s.seedEmail, string(hash), id)
	if errors.Is(err, errDuplicate) {
		return nil
	}
	return err
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/v1/signup", s.signup)
	mux.HandleFunc("POST /auth/v1/token", s.token)
	mux.HandleFunc("GET /auth/v1/user", s.getUser)
	mux.HandleFunc("PUT /auth/v1/user", s.updateUser)
	mux.HandleFunc("POST /auth/v1/logout", s.logout)
	mux.HandleFunc("POST /auth/v1/recover", s.recover)
	mux.HandleFunc("GET /auth/v1/verify", s.verify)
	mux.HandleFunc("GET /auth/v1/authorize", s.authorize)
	mux.HandleFunc("DELETE /auth/v1/admin/users/{id}", s.adminDelete)
	mux.HandleFunc("GET /auth/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	return cors(mux)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, apikey, x-client-info, x-supabase-api-version")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// authError は Supabase Auth と同じ形のエラー({code,error_code,msg})で返す。
func authError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"code": status, "error_code": code, "msg": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		authError(w, http.StatusBadRequest, "invalid_json", "リクエストの形式が正しくありません")
		return false
	}
	return true
}

func userJSON(u user) map[string]any {
	now := u.CreatedAt.UTC().Format(time.RFC3339)
	return map[string]any{
		"id":                 u.ID.String(),
		"aud":                "authenticated",
		"role":               "authenticated",
		"email":              u.Email,
		"email_confirmed_at": now,
		"app_metadata":       map[string]any{"provider": "email", "providers": []string{"email"}},
		"user_metadata":      map[string]any{},
		"identities": []map[string]any{{
			"identity_id": u.ID.String(),
			"id":          u.ID.String(),
			"user_id":     u.ID.String(),
			"identity_data": map[string]any{
				"email": u.Email, "email_verified": true, "sub": u.ID.String(),
			},
			"provider":   "email",
			"created_at": now,
			"updated_at": now,
		}},
		"created_at": now,
		"updated_at": now,
	}
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *server) accessToken(u user) (string, int64, error) {
	now := time.Now()
	exp := now.Add(time.Hour)
	claims := jwt.MapClaims{
		"sub":           u.ID.String(),
		"email":         u.Email,
		"aud":           "authenticated",
		"role":          "authenticated",
		"iss":           s.externalURL,
		"iat":           now.Unix(),
		"exp":           exp.Unix(),
		"app_metadata":  map[string]any{"provider": "email", "providers": []string{"email"}},
		"user_metadata": map[string]any{},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	return signed, exp.Unix(), err
}

func (s *server) session(ctx context.Context, u user) (map[string]any, error) {
	access, exp, err := s.accessToken(u)
	if err != nil {
		return nil, err
	}
	refresh, err := randomToken()
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveRefresh(ctx, u.ID, tokenHash(refresh), time.Now().Add(30*24*time.Hour)); err != nil {
		return nil, err
	}
	return map[string]any{
		"access_token":  access,
		"token_type":    "bearer",
		"expires_in":    3600,
		"expires_at":    exp,
		"refresh_token": refresh,
		"user":          userJSON(u),
	}, nil
}

func (s *server) bearerUser(r *http.Request) (user, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if raw == "" {
		return user{}, errNotFound
	}
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected method")
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return user{}, errNotFound
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return user{}, errNotFound
	}
	sub, _ := claims["sub"].(string)
	id, err := uuid.Parse(sub)
	if err != nil {
		return user{}, errNotFound
	}
	return s.store.UserByID(r.Context(), id)
}

func (s *server) signup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string         `json:"email"`
		Password string         `json:"password"`
		Data     map[string]any `json:"data"`
	}
	if !decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") || len(in.Password) < 6 {
		authError(w, http.StatusBadRequest, "validation_failed", "メールアドレスと6文字以上のパスワードを入れてください")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		authError(w, http.StatusInternalServerError, "unexpected_failure", "登録に失敗しました")
		return
	}
	u, err := s.store.CreateUser(r.Context(), email, string(hash), uuid.New())
	if errors.Is(err, errDuplicate) {
		// 登録済みでもアドレスの有無を隠す本番の挙動に合わせ、identities が空のユーザーを返す。
		writeJSON(w, http.StatusOK, map[string]any{
			"id": uuid.New().String(), "aud": "authenticated", "role": "authenticated",
			"email": email, "identities": []any{}, "created_at": time.Now().UTC().Format(time.RFC3339),
			"updated_at": time.Now().UTC().Format(time.RFC3339),
			"app_metadata": map[string]any{}, "user_metadata": map[string]any{},
		})
		return
	}
	if err != nil {
		authError(w, http.StatusInternalServerError, "unexpected_failure", "登録に失敗しました")
		return
	}
	// ローカルでは確認メールを送らないので、そのままログイン済みのセッションを返す。
	sess, err := s.session(r.Context(), u)
	if err != nil {
		authError(w, http.StatusInternalServerError, "unexpected_failure", "登録に失敗しました")
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *server) token(w http.ResponseWriter, r *http.Request) {
	grant := r.URL.Query().Get("grant_type")
	switch grant {
	case "password":
		var in struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if !decode(w, r, &in) {
			return
		}
		u, err := s.store.UserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(in.Email)))
		if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
			authError(w, http.StatusBadRequest, "invalid_credentials", "メールアドレスまたはパスワードが違います")
			return
		}
		sess, err := s.session(r.Context(), u)
		if err != nil {
			authError(w, http.StatusInternalServerError, "unexpected_failure", "ログインに失敗しました")
			return
		}
		writeJSON(w, http.StatusOK, sess)
	case "refresh_token":
		var in struct {
			RefreshToken string `json:"refresh_token"`
		}
		if !decode(w, r, &in) {
			return
		}
		id, err := s.store.ConsumeRefresh(r.Context(), tokenHash(in.RefreshToken))
		if err != nil {
			authError(w, http.StatusBadRequest, "refresh_token_not_found", "セッションの更新に失敗しました")
			return
		}
		u, err := s.store.UserByID(r.Context(), id)
		if err != nil {
			authError(w, http.StatusBadRequest, "user_not_found", "セッションの更新に失敗しました")
			return
		}
		sess, err := s.session(r.Context(), u)
		if err != nil {
			authError(w, http.StatusInternalServerError, "unexpected_failure", "セッションの更新に失敗しました")
			return
		}
		writeJSON(w, http.StatusOK, sess)
	default:
		authError(w, http.StatusBadRequest, "unsupported_grant_type", "対応していない grant_type です")
	}
}

func (s *server) getUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.bearerUser(r)
	if err != nil {
		authError(w, http.StatusUnauthorized, "invalid_token", "ログインが必要です")
		return
	}
	writeJSON(w, http.StatusOK, userJSON(u))
}

func (s *server) updateUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.bearerUser(r)
	if err != nil {
		authError(w, http.StatusUnauthorized, "invalid_token", "ログインが必要です")
		return
	}
	var in struct {
		Email    string         `json:"email"`
		Password string         `json:"password"`
		Data     map[string]any `json:"data"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Password != "" {
		if len(in.Password) < 6 {
			authError(w, http.StatusBadRequest, "validation_failed", "パスワードは6文字以上にしてください")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil || s.store.UpdatePassword(r.Context(), u.ID, string(hash)) != nil {
			authError(w, http.StatusInternalServerError, "unexpected_failure", "更新に失敗しました")
			return
		}
	}
	if in.Email != "" && !strings.EqualFold(in.Email, u.Email) {
		err := s.store.UpdateEmail(r.Context(), u.ID, strings.ToLower(strings.TrimSpace(in.Email)))
		if errors.Is(err, errDuplicate) {
			authError(w, http.StatusBadRequest, "email_exists", "そのメールアドレスは使われています")
			return
		}
		if err != nil {
			authError(w, http.StatusInternalServerError, "unexpected_failure", "更新に失敗しました")
			return
		}
		u.Email = strings.ToLower(strings.TrimSpace(in.Email))
	}
	writeJSON(w, http.StatusOK, userJSON(u))
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) recover(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	// アドレスの有無は明かさず、常に成功として返す(本番と同じ)。
	if u, err := s.store.UserByEmail(r.Context(), strings.ToLower(strings.TrimSpace(in.Email))); err == nil {
		if raw, err := randomToken(); err == nil {
			if err := s.store.SaveRecovery(r.Context(), u.ID, tokenHash(raw), time.Now().Add(time.Hour)); err == nil {
				// ローカルではメールを送らないので、再設定リンクをログに出して手元で開けるようにする。
				slog.Info("password recovery link (local only)",
					"url", s.externalURL+"/verify?type=recovery&token="+raw+"&redirect_to=http://localhost:3000/auth/callback")
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (s *server) verify(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("type") != "recovery" {
		authError(w, http.StatusBadRequest, "validation_failed", "対応していない type です")
		return
	}
	id, err := s.store.ConsumeRecovery(r.Context(), tokenHash(r.URL.Query().Get("token")))
	if err != nil {
		authError(w, http.StatusBadRequest, "otp_expired", "リンクの期限が切れています")
		return
	}
	u, err := s.store.UserByID(r.Context(), id)
	if err != nil {
		authError(w, http.StatusBadRequest, "user_not_found", "ユーザーが見つかりません")
		return
	}
	sess, err := s.session(r.Context(), u)
	if err != nil {
		authError(w, http.StatusInternalServerError, "unexpected_failure", "再設定に失敗しました")
		return
	}
	redirectTo := r.URL.Query().Get("redirect_to")
	if redirectTo == "" {
		redirectTo = "http://localhost:3000/reset-password/update"
	}
	target := fmt.Sprintf("%s#access_token=%s&expires_at=%v&expires_in=3600&refresh_token=%s&token_type=bearer&type=recovery",
		redirectTo, sess["access_token"], sess["expires_at"], sess["refresh_token"])
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *server) authorize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte("ローカル確認環境では Google ログインは使えません。メールアドレスとパスワードでログインしてください。"))
}

func (s *server) adminDelete(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("apikey")
	if s.serviceKey == "" || key != s.serviceKey {
		authError(w, http.StatusUnauthorized, "invalid_api_key", "権限がありません")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		authError(w, http.StatusBadRequest, "validation_failed", "id が正しくありません")
		return
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		authError(w, http.StatusInternalServerError, "unexpected_failure", "削除に失敗しました")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}
