package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/claude"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/mail"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/hinata/track-talk/apps/api/internal/stripe"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg       config.Config
	pool      *pgxpool.Pool
	q         *db.Queries
	auth      *auth.Verifier
	authAdmin auth.Admin
	limit     *RateLimiter
	cloud     cloudinary.Client
	push      push.Sender
	stripe    stripe.Client
	mail      mail.Client
	ai        claude.Client
}

func New(cfg config.Config, pool *pgxpool.Pool, v *auth.Verifier) http.Handler {
	s := &Server{
		cfg:  cfg,
		pool: pool,
		q:    db.New(pool),
		auth: v,
		authAdmin: auth.Admin{
			BaseURL: cfg.SupabaseURL,
			APIKey:  cfg.SupabaseServiceRoleKey,
		},
		limit: NewRateLimiter(v),
		cloud: cloudinary.Client{
			Cloud:        cfg.CloudName,
			Key:          cfg.CloudAPIKey,
			Secret:       cfg.CloudAPISecret,
			APIBase:      cfg.CloudAPIBase,
			AuthTokenKey: cfg.CloudAuthTokenKey,
		},
		push: push.Sender{
			PublicKey:  cfg.VAPIDPublicKey,
			PrivateKey: cfg.VAPIDPrivateKey,
			Subject:    cfg.VAPIDSubject,
		},
		stripe: stripe.Client{
			Secret:     cfg.StripeSecretKey,
			APIBase:    cfg.StripeAPIBase,
			APIVersion: cfg.StripeAPIVersion,
		},
		mail: mail.Client{
			APIKey:   cfg.ResendAPIKey,
			From:     cfg.MailFrom,
			APIBase:  cfg.MailAPIBase,
			SMTPHost: cfg.SMTPHost, SMTPPort: cfg.SMTPPort, SMTPUser: cfg.SMTPUser, SMTPPassword: cfg.SMTPPassword,
			DevLog: cfg.MailDevLog,
		},
		ai: claude.Client{
			MaxTokens: senseSummaryMaxTokens,
			Key:       cfg.AnthropicAPIKey,
			Model:     cfg.SenseSummaryModel,
			Endpoint:  cfg.AnthropicEndpoint,
		},
	}
	if cfg.PushHTTP != nil {
		s.push.HTTP = cfg.PushHTTP
	}
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Use(Recover)
	r.Use(RequestLog)
	// JSON を gzip で縮めて、Cloud Run から外への通信量を減らす。トークンはヘッダーで送り Cookie を使わないので、
	// 圧縮の長さから中身を当てる攻撃（BREACH）の前提は成り立たない。
	r.Use(middleware.Compress(5, "application/json"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(s.limit.Middleware)
	r.Get("/health", s.health)
	// Cloud Scheduler から1分ごとに呼ばれる。秘密のヘッダーで確かめる。
	// コーチングを提供していない間は、tick も無いものとして扱う。
	r.With(s.requireCoaching).Post("/internal/coaching/tick", s.coachingTick)
	// 支払いはコーチングの一部なので、提供していない間は Webhook も無いものとして扱う。
	r.With(s.requireCoaching).Post("/webhooks/stripe", s.stripeWebhook)
	// Cloud Scheduler から毎月1日に呼ばれる。秘密のヘッダーで確かめる。
	r.Post("/internal/monthly-reminder", s.monthlyReminder)
	r.Post("/internal/storage/cleanup", s.storageCleanup)

	r.Route("/v1", func(r chi.Router) {
		r.Use(OptionalAuth(v))
		r.Get("/posts", s.listPosts)
		r.Get("/posts/{id}", s.getPost)
		r.Get("/posts/{id}/comments", s.listComments)
		r.Get("/injury/stats", s.injuryStats)
		r.Get("/injury/data", s.injuryData)
		r.Get("/injury/groups", s.listInjuryGroups)
		r.Get("/injury/groups/{part}/public", s.getPublicInjuryGroup)
		r.Get("/products", s.listProducts)
		r.Get("/products/ranking", s.productRanking)
		r.Get("/products/{id}", s.getProduct)
		r.Get("/products/{id}/reviews", s.listProductReviews)
		r.Get("/menu-contest", s.getMenuContest)
		r.Get("/meet-threads/today", s.todayMeetThreads)
		r.Get("/meet-resources", s.listMeetResources)
		r.Get("/posts/{id}/meet-resources", s.getPostMeetResources)
		// プッシュ通知はコーチングの知らせと月1回の記録のお知らせに使う。コーチングの提供前でも登録できる。
		r.Get("/push/public-key", s.pushPublicKey)
		// 大会の日程と県ページは、検索から来た人がログインなしで読めるようにする。
		r.Get("/competitions", s.listCompetitions)
		r.Get("/major-hosts", s.listMajorHosts)
		r.Get("/domestic-majors", s.listDomesticMajors)
		r.Get("/prefectures/{code}", s.getPrefecturePage)

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(v))
			r.Get("/me", s.me)
			r.Patch("/me", s.updateProfile)
			r.Patch("/me/badge", s.updateBadge)
			r.Delete("/me", s.deleteAccount)
			r.Post("/me/consents/overseas", s.consentOverseas)
			r.Get("/me/practice-reputation", s.practiceReputation)
			r.Get("/me/posts", s.listMyPosts)
			r.Get("/me/injury-history", s.myInjuryHistory)
			r.Get("/injury/me/progress", s.myInjuryProgress)
			r.Get("/me/sense-notes", s.listSenseNotes)
			// 感覚メモの1週間のまとめ（AI）。提供を始めるまで 404。
			r.With(s.requireSenseSummary).Get("/me/sense-notes/summary", s.getSenseSummary)
			r.With(s.requireSenseSummary).Get("/me/sense-notes/summaries", s.listSenseSummaries)
			r.With(s.requireSenseSummary).Post("/me/sense-notes/summary", s.createSenseSummary)
			r.Post("/me/sense-notes", s.createSenseNote)
			r.Delete("/me/sense-notes/{id}", s.deleteSenseNote)
			r.Get("/me/race-records", s.listRaceRecords)
			r.Get("/race-records/pb-stats", s.pbStats)
			r.Post("/me/push-subscriptions", s.savePushSubscription)
			r.Delete("/me/push-subscriptions", s.deletePushSubscription)
			r.Post("/me/push-subscriptions/test", s.testPush)
			r.Post("/me/race-records", s.createRaceRecord)
			r.Patch("/me/race-records/{id}", s.updateRaceRecord)
			r.Delete("/me/race-records/{id}", s.deleteRaceRecord)
			r.Get("/me/target-meet", s.getTargetMeet)
			r.Put("/me/target-meet", s.setTargetMeet)
			r.Delete("/me/target-meet", s.deleteTargetMeet)
			r.Put("/menu-contest/entry", s.saveMenuContestEntry)
			r.Delete("/menu-contest/entry", s.deleteMenuContestEntry)
			r.Put("/menu-contest/vote", s.voteMenuContest)
			r.Get("/me/saves", s.listSaves)
			r.Get("/me/inquiries", s.listMyInquiries)
			r.Post("/me/inquiries", s.createInquiry)
			r.Get("/me/notifications", s.listNotifications)
			r.Get("/me/notification-settings", s.listNotificationSettings)
			r.Put("/me/notification-settings/{category}", s.updateNotificationSetting)
			r.Post("/me/notifications/read", s.markAllNotificationsRead)
			r.Post("/me/notifications/{id}/read", s.markNotificationRead)
			// コーチの応募は提供前の準備にも使うので閉じない。応募の状態は /coach/me で読み、
			// 顔写真は審査に必要なので応募の一部分として開けておく。
			r.Get("/coach/me", s.coachMe)
			r.Post("/coach/applications", s.submitCoachApplication)
			r.Put("/coach/me/photo", s.setCoachPhoto)
			r.Post("/posts", s.createPost)
			r.Post("/posts/youtube", s.citeYouTube)
			r.Post("/posts/instagram", s.citeInstagram)
			r.Post("/posts/x", s.citeXPost)
			r.Post("/uploads/video", s.signVideoUpload)
			r.Post("/uploads/image", s.signImageUpload)
			r.Patch("/posts/{id}", s.updatePost)
			r.Delete("/posts/{id}", s.deletePost)
			r.Post("/posts/{id}/likes", s.likePost)
			r.Delete("/posts/{id}/likes", s.unlikePost)
			r.Post("/posts/{id}/saves", s.savePost)
			r.Delete("/posts/{id}/saves", s.unsavePost)
			r.Post("/posts/{id}/comments", s.createComment)
			r.Get("/injury/groups/{part}", s.getInjuryGroup)
			r.Post("/injury/groups/{part}/join", s.joinInjuryGroup)
			r.Patch("/injury/groups/{part}/me", s.updateInjuryGroupStage)
			r.Delete("/injury/groups/{part}/me", s.leaveInjuryGroup)
			r.Get("/injury/groups/{part}/posts", s.listInjuryGroupPosts)
			r.Post("/injury/groups/{part}/posts", s.createInjuryGroupPost)
			r.Post("/posts/{id}/injury/updates", s.createInjuryUpdate)
			r.Patch("/posts/{id}/injury/updates/{updateID}", s.updateInjuryUpdate)
			r.Delete("/posts/{id}/injury/updates/{updateID}", s.deleteInjuryUpdate)
			r.Post("/posts/{id}/practice/applications", s.applyPractice)
			r.Post("/posts/{id}/practice/applications/{applicationID}/accept", s.acceptPractice)
			r.Post("/posts/{id}/practice/applications/{applicationID}/decline", s.declinePractice)
			r.Post("/posts/{id}/practice/applications/{applicationID}/rating", s.ratePractice)
			r.Post("/posts/{id}/practice/dissolve", s.dissolvePractice)
			r.Patch("/comments/{id}", s.updateComment)
			r.Delete("/comments/{id}", s.deleteComment)
			r.Post("/products/{id}/reviews", s.createProductReview)
			r.Post("/product-requests", s.createProductRequest)
			r.Delete("/product-reviews/{id}", s.deleteProductReview)
			r.Post("/reports", s.createReport)
			r.Get("/posts/{id}/my-reports", s.myReports)
			r.Get("/admin/reports", s.listReports)
			r.Post("/admin/reports/{id}/close", s.closeReport)
			r.Get("/admin/inquiries", s.listInquiries)
			r.Post("/admin/posts/{id}/hide", s.hidePost)
			r.Post("/admin/comments/{id}/hide", s.hideComment)
			r.Post("/admin/injury-notes/{id}/hide", s.hideInjuryNote)
			r.Post("/admin/practice-ratings/{id}/hide", s.hidePracticeRating)
			r.Get("/admin/meet-sessions", s.adminListMeetSessions)
			r.Put("/admin/meet-sessions", s.adminPutMeetSessions)
			r.Put("/admin/meet-resources", s.adminPutMeetResources)
			r.Post("/admin/products", s.createProduct)
			r.Patch("/admin/products/{id}", s.updateProduct)
			r.Post("/admin/products/{id}/hide", s.hideProduct)
			r.Post("/admin/product-reviews/{id}/hide", s.hideProductReview)
			r.Get("/admin/coach-applications", s.listCoachApplications)
			r.Post("/admin/coach-applications/{id}/approve", s.approveCoachApplication)
			r.Post("/admin/coach-applications/{id}/reject", s.rejectCoachApplication)
			r.Get("/admin/coaching/demo-codes", s.listDemoCodes)
			r.Post("/admin/coaching/demo-codes", s.createDemoCode)
			r.Get("/admin/coaching/probation", s.listCoachProbation)
			r.Post("/admin/coaching/responses/{id}/hide", s.hideReviewResponse)
			r.Post("/admin/coaching/coaches/{id}/reset-rating", s.resetCoachRating)
			r.Delete("/admin/coaching/coaches/{id}/photo", s.removeCoachPhoto)
			r.Get("/admin/coaching/contracts", s.adminListCoachingContracts)
			r.Get("/admin/coaching/contracts/{id}", s.adminOpenCoachingContract)
			r.Post("/admin/coaching/contracts/{id}/suspend", s.adminSuspendCoachingContract)
			r.Post("/admin/coaching/contracts/{id}/resume", s.adminResumeCoachingContract)
			r.Get("/admin/menu-contest", s.adminMenuContest)
			r.Get("/admin/stats", s.adminStats)
			r.Get("/admin/stats/coaching", s.adminCoachingStats)
			r.Put("/admin/menu-contest/finalists", s.setMenuContestFinalists)
			r.Get("/admin/announcements", s.adminListAnnouncements)
			r.Get("/admin/announcements/recipients", s.adminAnnouncementRecipients)
			r.Post("/admin/announcements/test", s.adminTestAnnouncement)
			r.Post("/admin/announcements", s.adminSendAnnouncement)
			r.Post("/admin/announcements/{id}/cancel", s.adminCancelAnnouncement)
		})

		// オンラインコーチング（年齢・保護者の同意・プッシュ通知・添削動画・パーソナル指導・
		// フォーム添削・コーチの報酬と送金）は提供前なので、COACHING_ENABLED が立っていないと
		// 404 を返す。プライバシーポリシーもその前提で書いている。
		// 閉じないもの: コーチの応募（POST /coach/applications、GET /coach/me、審査に必要な顔写真）と、
		// 運営の審査・準備に使う /admin/coach-applications、/admin/coaching の閲覧系。
		// お金の動き（送金・返金）は閉じる。
		r.Group(func(r chi.Router) {
			r.Use(s.requireCoaching)
			r.Get("/coaches", s.listCoaches)
			r.Get("/coaching/plans/{id}", s.getPublicCoachingPlan)
			// 保護者の同意のページ。ログインはいらず、リンクのトークンで本人を確かめる。
			r.Get("/guardian/{token}", s.getGuardian)
			r.Post("/guardian/{token}/consent", s.consentGuardian)
			r.Patch("/guardian/{token}", s.updateGuardian)
			r.Post("/guardian/{token}/decline", s.declineGuardian)
			r.Post("/guardian/{token}/revoke", s.revokeGuardian)
			r.Get("/guardian/{token}/contracts", s.listGuardianContracts)
			r.Get("/guardian/{token}/contracts/{id}", s.getGuardianContract)
			r.Post("/guardian/{token}/contracts/{id}/consent", s.consentGuardianContract)
			r.Post("/guardian/{token}/contracts/{id}/decline", s.declineGuardianContract)
			r.Post("/guardian/{token}/contracts/{id}/checkout", s.checkoutGuardianContract)
			r.Get("/guardian/{token}/contracts/{id}/support", s.getCoachingSupport)
			r.Get("/guardian/{token}/contracts/{id}/in-person", s.getInPerson)
			r.Post("/guardian/{token}/contracts/{id}/in-person/{cid}/{action}", s.actInPersonSession)
			r.Post("/guardian/{token}/contracts/{id}/calls/{cid}/{action}", s.actSupportCall)
			r.Get("/guardian/{token}/contracts/{id}/messages", s.listGuardianMessages)
			r.Post("/guardian/{token}/contracts/{id}/messages", s.createGuardianMessage)
			r.Post("/guardian/{token}/messages/{mid}/report", s.reportGuardianMessage)
			r.Get("/guardian/{token}/consultation-reviews", s.listGuardianConsultationReviews)
			r.Get("/guardian/{token}/contracts/{id}/consultations", s.listConsultationRoom)
			r.Get("/guardian/{token}/reviews/{id}/consultations", s.listConsultationRoom)
			r.With(s.requireNewAcceptance).Post("/guardian/{token}/consultations/{id}/checkout", s.checkoutConsultation)
			r.Post("/guardian/{token}/consultations/{id}/report", s.reportGuardianConsultation)

			r.Group(func(r chi.Router) {
				r.Use(RequireAuth(v))
				// 利用者の準備（年齢・保護者リンク・プッシュ通知・添削用の動画）
				r.Get("/me/coaching", s.coachingMe)
				r.Put("/me/birth-month", s.setBirthMonth)
				r.Post("/me/guardian-link", s.issueGuardianLink)
				r.Post("/uploads/coaching-video", s.signCoachingVideoUpload)
				// パーソナル指導（利用者側）
				r.With(s.requireNewAcceptance).Post("/coaching/contracts", s.createCoachingContract)
				r.Get("/coaching/contracts/{id}", s.getCoachingContract)
				r.Post("/coaching/contracts/{id}/cancel", s.cancelCoachingContract)
				r.Post("/coaching/contracts/{id}/checkout", s.checkoutCoachingContract)
				r.Get("/coaching/contracts/{id}/support", s.getCoachingSupport)
				r.Get("/coaching/contracts/{id}/in-person", s.getInPerson)
				r.Post("/coaching/contracts/{id}/in-person", s.createInPersonSession)
				r.Post("/coaching/contracts/{id}/in-person/{cid}/{action}", s.actInPersonSession)
				r.Post("/coaching/contracts/{id}/calls", s.createSupportCall)
				r.Post("/coaching/contracts/{id}/calls/{cid}/{action}", s.actSupportCall)
				r.Get("/coaching/contracts/{id}/messages", s.listCoachingMessages)
				r.Post("/coaching/contracts/{id}/messages", s.createCoachingMessage)
				r.Post("/coaching/contracts/{id}/rating", s.rateCoachingContract)
				r.Get("/me/coaching/contracts", s.listMyCoachingContracts)
				// フォーム添削（利用者側）
				r.Get("/coaching/estimate", s.reviewEstimate)
				r.Post("/coaching/demo", s.joinCoachingDemo)
				r.With(s.requireNewAcceptance).Post("/coaching/requests", s.createReviewRequest)
				r.Get("/coaching/requests/{id}/consultations", s.listConsultationRoom)
				r.Post("/coaching/requests/{id}/consultations", s.saveConsultationDraft)
				r.Get("/coaching/contracts/{id}/consultations", s.listConsultationRoom)
				r.Post("/coaching/contracts/{id}/consultations", s.saveConsultationDraft)
				r.With(s.requireNewAcceptance).Post("/coaching/consultations/{id}/submit", s.submitConsultation)
				r.With(s.requireNewAcceptance).Post("/coaching/consultations/{id}/checkout", s.checkoutConsultation)
				r.Post("/coaching/consultations/{id}/{action}", s.actConsultation)
				r.Get("/coach/consultations", s.coachConsultations)
				r.Get("/admin/coaching/consultations", s.adminConsultations)
				r.Get("/admin/coaching/consultations/{id}", s.adminOpenConsultation)
				r.Post("/admin/coaching/consultations/{id}/hide", s.adminHideConsultation)
				r.Post("/admin/coaching/consultations/{id}/refund", s.adminRefundConsultation)
				r.Get("/me/coaching/tickets", s.myTickets)
				r.With(s.requireNewAcceptance).Post("/coaching/tickets/checkout", s.checkoutTickets)
				r.Get("/coaching/tickets/purchases/{id}", s.myTicketPurchase)
				r.Get("/admin/coaching/tickets", s.adminTickets)
				r.Get("/admin/coaching/reconciliation", s.adminReconciliation)
				r.Post("/admin/coaching/tickets/{id}/refund", s.adminRefundTickets)
				r.With(s.requireNewAcceptance).Post("/coaching/requests/{id}/checkout", s.checkoutReviewRequest)
				r.Get("/me/coaching/requests", s.listMyReviewRequests)
				r.Get("/coaching/requests/{id}", s.getReviewRequest)
				r.Post("/coaching/requests/{id}/clarify", s.clarifyReview)
				r.Post("/coaching/requests/{id}/rating", s.rateReview)
				r.Post("/coaching/requests/{id}/helpful", s.answerHelpful)
				// コーチ側（プロフィール・報酬・プラン・契約・添削の返却）
				// 顔写真（PUT /coach/me/photo）は応募の審査に使うので、閉じない方に置いてある。
				r.Patch("/coach/me", s.updateCoachDuty)
				r.Post("/coach/me/pledge", s.pledgeCoach)
				r.Patch("/coach/me/services", s.updateCoachServices)
				r.Get("/coach/me/earnings", s.coachEarnings)
				r.Post("/coach/stripe/onboarding", s.startCoachOnboarding)
				r.Get("/coach/me/plans", s.listMyCoachingPlans)
				r.Post("/coach/me/plans", s.createCoachingPlan)
				r.Patch("/coach/me/plans/{id}", s.updateCoachingPlan)
				r.Get("/coach/contracts", s.listCoachContracts)
				r.Post("/coach/contracts/{id}/accept", s.acceptCoachingContract)
				r.Post("/coach/contracts/{id}/decline", s.declineCoachingContract)
				r.Get("/coach/requests", s.listCoachRequests)
				r.Get("/coach/requests/{id}", s.getCoachReviewRequest)
				r.Post("/coaching/requests/{id}/response", s.submitReviewResponse)
				r.Post("/coaching/requests/{id}/clarification", s.submitClarification)
				r.Post("/coach/offers/{id}/accept", s.acceptReviewOffer)
				r.Post("/coach/offers/{id}/decline", s.declineReviewOffer)
				// 運営のお金の動き（送金と返金）
				r.Get("/admin/coaching/payouts", s.adminCoachPayouts)
				r.Post("/admin/coaching/payouts/run", s.adminRunCoachPayouts)
				r.Post("/admin/coaching/contracts/{id}/refund", s.adminRefundCoachingContract)
			})
		})
	})
	return r
}

// requireNewAcceptance は、新規受付を止めている間、新しい申し込み・購入だけを断る。
// 既存の契約・会話・返金・精算はこの対象にしない。
func (s *Server) requireNewAcceptance(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.CoachingNewAcceptance {
			Error(w, http.StatusConflict, "new_acceptance_closed", "新しい申し込みは準備中です")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireCoaching は、オンラインコーチングを提供していない間、その API を無いものとして扱う。
func (s *Server) requireCoaching(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.CoachingEnabled {
			Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// now は現在時刻。テストでは設定の Clock で差し替える。
func (s *Server) now() time.Time {
	if s.cfg.Clock != nil {
		return s.cfg.Clock()
	}
	return time.Now()
}

func (s *Server) present(p postDTO) postDTO {
	if p.Kind == "video" && p.VideoPublicID != "" {
		// ローカルのプレビュー用サンプルは外部ストレージに接続しない。
		if name, ok := strings.CutPrefix(p.VideoPublicID, "local/"); ok && s.cfg.LocalMediaBaseURL != "" {
			p.ThumbnailURL = s.cfg.LocalMediaBaseURL + "/" + name + ".jpg"
			p.PlaybackURL = s.cfg.LocalMediaBaseURL + "/" + name + ".mp4"
			return p
		}
		if p.list {
			p.ThumbnailURL = s.cloud.CardThumbnailURL(p.VideoPublicID)
		} else {
			p.ThumbnailURL = s.cloud.ThumbnailURL(p.VideoPublicID)
		}
		p.PlaybackURL = s.cloud.PlaybackURL(p.VideoPublicID)
	}
	if (p.Kind == "text" || p.Kind == "recruitment" || p.Kind == "instagram" || p.Kind == "x") && p.ImagePublicID != "" {
		if p.list {
			p.ThumbnailURL = s.cloud.CardImageURL(p.ImagePublicID)
		} else {
			p.ThumbnailURL = s.cloud.ImageURL(p.ImagePublicID)
		}
	}
	return p
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		Error(w, http.StatusServiceUnavailable, "unhealthy", "データベースに接続できません")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
