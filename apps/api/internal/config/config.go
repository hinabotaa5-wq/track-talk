package config

import (
	"fmt"
	"net/http"
	"net/url"
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
	// CoachingConsultation が false の間は、動画添削のあとの追加相談（1つの添削につき質問1回・回答1回の
	// 一方通行の相談）の送信系（質問の送信・支払い・コーチの回答）を止める。下書き・閲覧・通報・返金・取消しは止めない。
	// 契約の追加相談・やりとり欄・添削の確認質問は CoachingMessaging の側に残す。既定は停止。
	CoachingConsultation bool
	// CoachingCoachList が false の間は、コーチ一覧 API（GET /v1/coaches）をフラグ OFF のときと同じ 404 にし、
	// コーチの公開プロフィールを一覧から取れないようにする。CoachingEnabled が false なら値によらず閉じる。
	// 動画添削の指名候補（GET /v1/coaching/nominees）と申込みは止めない。既定は停止。
	CoachingCoachList bool
	// CoachingTicketSales が false の間は、コーチングチケットの新規購入を止める。動画添削は都度払い
	// （今はカードのみ）だけにしたので既定は停止。購入済みチケットの残高・履歴・返金は止めない。
	// 再開するときだけ立てる。テーブルと過去の購入・消費の記録は残す。
	CoachingTicketSales bool
	// ReviewRandomAssign が false の間は、動画添削の「おまかせ」(運営が受付中のコーチへ順番に割り当てる)を止め、
	// 選手が選んだコーチにだけ依頼を届ける。既定は停止。割り当てのコードと過去の打診・割り当ての記録は残す。
	ReviewRandomAssign bool
	// IdentityReaderURL and IdentityReaderToken authorize only publication reads.
	IdentityReaderURL   string
	IdentityReaderToken string
	// IdentityEndToken authorizes only end-of-coaching recording on the
	// dedicated identity service. It never grants identity reads or publication writes.
	IdentityEndToken           string
	DatabaseMigrationsExternal bool
	// CoachingTickSecret は Cloud Scheduler が添削の tick を呼ぶときのヘッダーの値。空なら tick は使えない。
	CoachingTickSecret string
	// StorageCleanupSecret は日次の保存期限処理に使う。空なら呼び出せない。
	StorageCleanupSecret string
	// ReminderSecret は Cloud Scheduler が月1回の記録のお知らせを呼ぶときのヘッダーの値。空なら使えない。
	ReminderSecret string
	// SecurityCheckSecret は Cloud Scheduler が見慣れない DB 接続の見回りを呼ぶときのヘッダーの値。空なら使えない。
	SecurityCheckSecret string
	// OpsAlertEmail は、見覚えのないお金の動きや DB 接続などを運営へ知らせる宛先。秘密情報ではない。空ならメールは送らない。
	OpsAlertEmail string
	// OpsPayoutExpectedLast4 は、Stripe から運営の銀行へ入金されるときの宛先口座の末尾4桁（数字4桁だけ）。
	// 秘密情報ではない。空なら入金の宛先チェックはしない。実際の値はコードや設定例に書かない。
	OpsPayoutExpectedLast4 string
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
	// StaffMFARequired が true なら、管理者の API は二段階認証(aal2)を済ませたログインでしか使えない。
	// 必須は管理者だけで、コーチには求めない。管理者が二段階認証を設定し終えるまで、本番では立てない。
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
		CoachingConsultation:       enabled(os.Getenv("COACHING_CONSULTATION_ENABLED")),
		CoachingCoachList:          enabled(os.Getenv("COACHING_COACH_LIST_ENABLED")),
		CoachingTicketSales:        enabled(os.Getenv("COACHING_TICKET_SALES_ENABLED")),
		ReviewRandomAssign:         enabled(os.Getenv("REVIEW_RANDOM_ASSIGN_ENABLED")),
		IdentityReaderURL:          os.Getenv("IDENTITY_READER_URL"),
		IdentityReaderToken:        os.Getenv("IDENTITY_PUBLIC_READER_TOKEN"),
		IdentityEndToken:           os.Getenv("IDENTITY_END_TOKEN"),
		DatabaseMigrationsExternal: enabled(os.Getenv("DATABASE_MIGRATIONS_EXTERNAL")),
		CoachingTickSecret:         os.Getenv("COACHING_TICK_SECRET"),
		StorageCleanupSecret:       os.Getenv("STORAGE_CLEANUP_SECRET"),
		ReminderSecret:             os.Getenv("REMINDER_SECRET"),
		SecurityCheckSecret:        os.Getenv("SECURITY_CHECK_SECRET"),
		OpsAlertEmail:              strings.TrimSpace(os.Getenv("OPS_ALERT_EMAIL")),
		OpsPayoutExpectedLast4:     strings.TrimSpace(os.Getenv("OPS_PAYOUT_EXPECTED_LAST4")),
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
	if cfg.IdentityReaderURL != "" {
		if _, legacy := os.LookupEnv("COACH_IDENTITY_KEY"); legacy {
			return Config{}, fmt.Errorf("remove the legacy identity key from the API environment")
		}
		u, err := url.Parse(cfg.IdentityReaderURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" ||
			len(cfg.IdentityReaderToken) < 32 || len(cfg.IdentityEndToken) < 32 || !cfg.DatabaseMigrationsExternal {
			return Config{}, fmt.Errorf("identity reader requires an HTTPS origin, read token, end token and separately managed migrations")
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
