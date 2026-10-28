package config

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// 無料枠との比較（2026-10-27: Flash-Lite は500回/日・15回/分）に余裕を残す。
const (
	DefaultSenseSummaryDailyLimit  = 400
	DefaultSenseSummaryMinuteLimit = 10
)

type Config struct {
	Addr                   string
	DatabaseURL            string
	SupabaseURL            string
	SupabaseServiceRoleKey string
	JWTSecret              string
	JWTAUD                 string
	CORSOrigins            []string
	LogLevel               string
	ShutdownTimeout        time.Duration
	CloudName              string
	CloudAPIKey            string
	CloudAPISecret         string
	CloudAPIBase           string
	CloudAuthTokenKey      string // Cloudinary のトークン認証の鍵。あれば非公開の動画の URL に期限を付ける。
	VAPIDPublicKey         string
	VAPIDPrivateKey        string
	VAPIDSubject           string
	// CoachingEnabled が false の間は、オンラインコーチングの API（年齢・保護者の同意・
	// プッシュ通知・添削動画・パーソナル指導・コーチの報酬）を 404 にする。
	// 提供を始めるまで本番では立てない。
	CoachingEnabled bool
	// CoachingNewAcceptance が false の間は、新しい申し込み・購入だけを止める。
	// 既存の契約・会話・決済の処理は CoachingEnabled の側で続ける。
	CoachingNewAcceptance bool
	// CoachingMessaging が false の間は、有料の1対1のやりとり（契約のやりとり欄・追加相談・
	// 添削の「わからなかった」と補足回答）の送信を止める。読む・通報・返金は止めない。
	// 有料の1対1メッセージは電気通信事業の届出の対象なので、届出が受理されるまで本番では立てない。
	CoachingMessaging bool
	// CoachIdentityKey は、コーチの本名・住所・電話番号を暗号化する鍵(32バイトを base64 にしたもの)。
	// DB とは別に Secret Manager などで持つ。空なら、コーチの特商法の登録情報は保存も表示もできない。
	CoachIdentityKey string
	// CoachingTickSecret は Cloud Scheduler が添削の tick を呼ぶときのヘッダーの値。空なら tick は使えない。
	CoachingTickSecret string
	// StorageCleanupSecret は日次の保存期限処理に使う。空なら呼び出せない。
	StorageCleanupSecret string
	// ReminderSecret は Cloud Scheduler が月1回の記録のお知らせを呼ぶときのヘッダーの値。空なら使えない。
	ReminderSecret string
	// Stripe の鍵。秘密鍵が空なら、パーソナル指導は支払いなしのデモ版として動く。
	StripeSecretKey     string
	StripeWebhookSecret string
	// Connect（コーチのアカウント）の知らせを受ける Webhook の署名の鍵。Stripe は送り先ごとに鍵を分けるので、別に作ったときだけ入れる。
	StripeConnectWebhookSecret string
	// StripeAPIBase はテストで Stripe の代わりのサーバーに向けるためのもの。
	StripeAPIBase string
	// StripeAPIVersion を入れると、Stripe の API のバージョンを固定する(空ならアカウントの既定)。
	StripeAPIVersion string
	// WebBaseURL は、支払い画面から戻ってくる Web の URL。空なら CORS_ORIGINS の最初のもの。
	WebBaseURL string
	// Resend の API キーと送り主。保護者の同意のリンクを保護者のメールあてに送る。空なら送れない（リンクも出せない）。
	ResendAPIKey string
	MailFrom     string
	// SMTP で送るときの設定(Gmail なら smtp.gmail.com / 465 / アプリパスワード)。SMTPHost が入っていれば Resend より優先する。
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	// MailAPIBase はテストで Resend の代わりのサーバーに向けるためのもの。
	MailAPIBase string
	// MailDevLog が true なら、メールを送らずに本文を API のログに出す。手元で試すためのもので、本番では立てない。
	MailDevLog bool
	// SenseSummaryEnabled が false の間は、感覚メモの1週間のまとめ（AI）の API を 404 にする。
	// 感覚メモを Claude(Anthropic)に送るので、プライバシーポリシーに委託先を書くまで本番では立てない。
	SenseSummaryEnabled bool
	// StaffMFARequired が true なら、管理者と承認済みコーチの API は二段階認証(aal2)を済ませたログインでしか使えない。
	// 管理者・コーチが二段階認証を設定し終えるまで、本番では立てない。
	StaffMFARequired bool
	// Anthropic の API キー。感覚メモのまとめに使う。空ならまとめは作れない。
	AnthropicAPIKey string
	// SenseSummaryModel は、感覚メモのまとめに使う Claude のモデル。空なら claude.DefaultModel(Haiku)。
	SenseSummaryModel string
	// SenseSummaryDailyLimit は、全員で直近24時間に試行できる要約の数。未設定は400、明示的な0は上限なし。
	// 有料の API なので、使われすぎて料金が膨らまないための上限。
	// 成否を問わず呼び出し直前にDBで予約する。日付のリセット時刻の違いや同時リクエストでも超えない。
	SenseSummaryDailyLimit int
	// SenseSummaryMinuteLimit は、全員で直近1分に試行できる数。未設定は10、明示的な0は上限なし。
	SenseSummaryMinuteLimit int
	// LocalMediaBaseURL はローカルプレビューの動画配信先。本番では設定しない。
	LocalMediaBaseURL string
	// AnthropicEndpoint はテストで Claude の代わりのサーバーに向けるためのもの。
	AnthropicEndpoint string
	// PushHTTP はテストで Web Push の送信先を差し替えるためのもの。nil なら既定のクライアント。
	PushHTTP *http.Client
	// Clock はテストで現在時刻を差し替えるためのもの。nil なら time.Now。
	Clock func() time.Time
}

func Load() (Config, error) {
	_ = godotenv.Load()
	_ = godotenv.Load("../../.env")

	cfg := Config{
		Addr:                       env("API_ADDR", ":8080"),
		DatabaseURL:                os.Getenv("DATABASE_URL"),
		SupabaseURL:                os.Getenv("SUPABASE_URL"),
		SupabaseServiceRoleKey:     os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		JWTSecret:                  os.Getenv("SUPABASE_JWT_SECRET"),
		JWTAUD:                     env("SUPABASE_JWT_AUD", "authenticated"),
		LogLevel:                   env("LOG_LEVEL", "info"),
		ShutdownTimeout:            10 * time.Second,
		CloudName:                  os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudAPIKey:                os.Getenv("CLOUDINARY_API_KEY"),
		CloudAPISecret:             os.Getenv("CLOUDINARY_API_SECRET"),
		CloudAPIBase:               os.Getenv("CLOUDINARY_API_BASE"),
		CloudAuthTokenKey:          os.Getenv("CLOUDINARY_AUTH_TOKEN_KEY"),
		VAPIDPublicKey:             os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:            os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:               os.Getenv("VAPID_SUBJECT"),
		CoachingEnabled:            enabled(os.Getenv("COACHING_ENABLED")),
		CoachingNewAcceptance:      enabled(os.Getenv("COACHING_NEW_ACCEPTANCE_ENABLED")),
		CoachingMessaging:          enabled(os.Getenv("COACHING_MESSAGING_ENABLED")),
		CoachIdentityKey:           os.Getenv("COACH_IDENTITY_KEY"),
		CoachingTickSecret:         os.Getenv("COACHING_TICK_SECRET"),
		StorageCleanupSecret:       os.Getenv("STORAGE_CLEANUP_SECRET"),
		ReminderSecret:             os.Getenv("REMINDER_SECRET"),
		StripeSecretKey:            os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:        os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripeConnectWebhookSecret: os.Getenv("STRIPE_CONNECT_WEBHOOK_SECRET"),
		StripeAPIBase:              os.Getenv("STRIPE_API_BASE"),
		StripeAPIVersion:           os.Getenv("STRIPE_API_VERSION"),
		WebBaseURL:                 strings.TrimRight(os.Getenv("WEB_BASE_URL"), "/"),
		ResendAPIKey:               os.Getenv("RESEND_API_KEY"),
		MailFrom:                   os.Getenv("MAIL_FROM"),
		MailAPIBase:                os.Getenv("MAIL_API_BASE"),
		SMTPHost:                   os.Getenv("SMTP_HOST"),
		SMTPPort:                   os.Getenv("SMTP_PORT"),
		SMTPUser:                   os.Getenv("SMTP_USER"),
		SMTPPassword:               os.Getenv("SMTP_PASSWORD"),
		MailDevLog:                 enabled(os.Getenv("MAIL_DEV_LOG")),
		SenseSummaryEnabled:        enabled(os.Getenv("SENSE_SUMMARY_ENABLED")),
		StaffMFARequired:           enabled(os.Getenv("STAFF_MFA_REQUIRED")),
		LocalMediaBaseURL:          strings.TrimRight(os.Getenv("LOCAL_MEDIA_BASE_URL"), "/"),
		AnthropicAPIKey:            os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicEndpoint:          os.Getenv("ANTHROPIC_ENDPOINT"),
		SenseSummaryModel:          strings.TrimSpace(os.Getenv("SENSE_SUMMARY_MODEL")),
		SenseSummaryDailyLimit:     DefaultSenseSummaryDailyLimit,
		SenseSummaryMinuteLimit:    DefaultSenseSummaryMinuteLimit,
	}
	for name, target := range map[string]*int{
		"SENSE_SUMMARY_DAILY_LIMIT":  &cfg.SenseSummaryDailyLimit,
		"SENSE_SUMMARY_MINUTE_LIMIT": &cfg.SenseSummaryMinuteLimit,
	} {
		if raw := strings.TrimSpace(os.Getenv(name)); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				return Config{}, fmt.Errorf("%s は0以上の整数にしてください: %q", name, raw)
			}
			*target = n
		}
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" && cfg.SupabaseURL == "" {
		return Config{}, fmt.Errorf("SUPABASE_JWT_SECRET or SUPABASE_URL is required")
	}
	origins := env("CORS_ORIGINS", "http://localhost:3000")
	for _, o := range strings.Split(origins, ",") {
		if v := strings.TrimSpace(o); v != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, v)
		}
	}
	if cfg.WebBaseURL == "" && len(cfg.CORSOrigins) > 0 {
		cfg.WebBaseURL = strings.TrimRight(cfg.CORSOrigins[0], "/")
	}
	if err := cfg.ValidateStripe(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// StripeMode は、秘密鍵の種類。鍵が無ければ空(支払いなしのデモ版)、テスト用の鍵なら "test"、本番の鍵なら "live"。
func (c Config) StripeMode() string {
	switch {
	case c.StripeSecretKey == "":
		return ""
	case strings.Contains(c.StripeSecretKey, "_live_"):
		return "live"
	}
	return "test"
}

// ValidateStripe は、Stripe の設定の取り違えを起動のときに止める。鍵だけ入れて Webhook の鍵を入れ忘れると、
// お金は動くのに期間が始まらない状態になるため。秘密鍵が空なら何も確かめない。
func (c Config) ValidateStripe() error {
	if c.StripeSecretKey == "" {
		return nil
	}
	if !strings.HasPrefix(c.StripeSecretKey, "sk_") && !strings.HasPrefix(c.StripeSecretKey, "rk_") {
		return fmt.Errorf("STRIPE_SECRET_KEY must be a secret key (sk_...) or restricted key (rk_...), not a publishable key")
	}
	if c.StripeWebhookSecret == "" {
		return fmt.Errorf("STRIPE_WEBHOOK_SECRET is required when STRIPE_SECRET_KEY is set (payments would never complete)")
	}
	for name, v := range map[string]string{
		"STRIPE_WEBHOOK_SECRET":         c.StripeWebhookSecret,
		"STRIPE_CONNECT_WEBHOOK_SECRET": c.StripeConnectWebhookSecret,
	} {
		if v != "" && !strings.HasPrefix(v, "whsec_") {
			return fmt.Errorf("%s must be a signing secret (whsec_...)", name)
		}
	}
	if c.StripeMode() == "live" {
		if !strings.HasPrefix(c.WebBaseURL, "https://") {
			return fmt.Errorf("WEB_BASE_URL (or the first CORS_ORIGINS) must be https:// when using a live Stripe key")
		}
		if c.CoachingTickSecret == "" {
			return fmt.Errorf("COACHING_TICK_SECRET is required when using a live Stripe key (expiry and refunds run in the tick)")
		}
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func enabled(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
