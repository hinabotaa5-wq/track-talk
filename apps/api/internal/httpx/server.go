package httpx

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
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
		limit: NewRateLimiter(),
		cloud: cloudinary.Client{
			Cloud:   cfg.CloudName,
			Key:     cfg.CloudAPIKey,
			Secret:  cfg.CloudAPISecret,
			APIBase: cfg.CloudAPIBase,
		},
		push: push.Sender{
			PublicKey:  cfg.VAPIDPublicKey,
			PrivateKey: cfg.VAPIDPrivateKey,
			Subject:    cfg.VAPIDSubject,
		},
		stripe: stripe.Client{
			Secret:  cfg.StripeSecretKey,
			APIBase: cfg.StripeAPIBase,
		},
	}
	if cfg.PushHTTP != nil {
		s.push.HTTP = cfg.PushHTTP
	}
	r := chi.NewRouter()
	r.Use(SecurityHeaders)
	r.Use(Recover)
	r.Use(RequestLog)
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
	r.Post("/internal/coaching/tick", s.coachingTick)
	r.Post("/webhooks/stripe", s.stripeWebhook)

	r.Route("/v1", func(r chi.Router) {
		r.Use(OptionalAuth(v))
		r.Get("/posts", s.listPosts)
		r.Get("/posts/{id}", s.getPost)
		r.Get("/posts/{id}/comments", s.listComments)
		r.Get("/injury/stats", s.injuryStats)
		r.Get("/injury/groups", s.listInjuryGroups)
		r.Get("/injury/groups/{part}/public", s.getPublicInjuryGroup)
		r.Get("/products", s.listProducts)
		r.Get("/products/{id}", s.getProduct)
		r.Get("/products/{id}/reviews", s.listProductReviews)
		r.Get("/push/public-key", s.pushPublicKey)
		r.Get("/coaches", s.listCoaches)
		r.Get("/coaching/plans/{id}", s.getPublicCoachingPlan)
		r.Get("/menu-contest", s.getMenuContest)
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
		r.Get("/guardian/{token}/contracts/{id}/messages", s.listGuardianMessages)
		r.Post("/guardian/{token}/contracts/{id}/messages", s.createGuardianMessage)
		r.Post("/guardian/{token}/messages/{mid}/report", s.reportGuardianMessage)

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(v))
			r.Get("/competitions", s.listCompetitions)
			r.Get("/major-hosts", s.listMajorHosts)
			r.Get("/domestic-majors", s.listDomesticMajors)
			r.Get("/me", s.me)
			r.Patch("/me", s.updateProfile)
			r.Delete("/me", s.deleteAccount)
			r.Get("/me/practice-reputation", s.practiceReputation)
			r.Get("/me/posts", s.listMyPosts)
			r.Get("/me/injury-history", s.myInjuryHistory)
			r.Get("/me/sense-notes", s.listSenseNotes)
			r.Post("/me/sense-notes", s.createSenseNote)
			r.Patch("/me/sense-notes/{id}", s.updateSenseNote)
			r.Delete("/me/sense-notes/{id}", s.deleteSenseNote)
			r.Get("/me/race-records", s.listRaceRecords)
			r.Post("/me/race-records", s.createRaceRecord)
			r.Patch("/me/race-records/{id}", s.updateRaceRecord)
			r.Delete("/me/race-records/{id}", s.deleteRaceRecord)
			r.Put("/menu-contest/entry", s.saveMenuContestEntry)
			r.Delete("/menu-contest/entry", s.deleteMenuContestEntry)
			r.Put("/menu-contest/vote", s.voteMenuContest)
			r.Get("/me/saves", s.listSaves)
			r.Get("/me/inquiries", s.listMyInquiries)
			r.Post("/me/inquiries", s.createInquiry)
			r.Get("/me/notifications", s.listNotifications)
			r.Post("/me/notifications/read", s.markAllNotificationsRead)
			r.Post("/me/notifications/{id}/read", s.markNotificationRead)
			r.Get("/me/coaching", s.coachingMe)
			r.Put("/me/birth-month", s.setBirthMonth)
			r.Post("/me/guardian-link", s.issueGuardianLink)
			r.Post("/me/push-subscriptions", s.savePushSubscription)
			r.Delete("/me/push-subscriptions", s.deletePushSubscription)
			r.Post("/me/push-subscriptions/test", s.testPush)
			r.Get("/coach/me", s.coachMe)
			r.Post("/coach/applications", s.submitCoachApplication)
			r.Patch("/coach/me", s.updateCoachDuty)
			r.Put("/coach/me/photo", s.setCoachPhoto)
			r.Post("/coach/me/pledge", s.pledgeCoach)
			r.Get("/coach/me/plans", s.listMyCoachingPlans)
			r.Post("/coach/me/plans", s.createCoachingPlan)
			r.Patch("/coach/me/plans/{id}", s.updateCoachingPlan)
			r.Get("/coach/contracts", s.listCoachContracts)
			r.Post("/coach/contracts/{id}/accept", s.acceptCoachingContract)
			r.Post("/coach/contracts/{id}/decline", s.declineCoachingContract)
			r.Post("/coaching/contracts", s.createCoachingContract)
			r.Get("/coaching/contracts/{id}", s.getCoachingContract)
			r.Post("/coaching/contracts/{id}/cancel", s.cancelCoachingContract)
			r.Post("/coaching/contracts/{id}/checkout", s.checkoutCoachingContract)
			r.Get("/coaching/contracts/{id}/messages", s.listCoachingMessages)
			r.Post("/coaching/contracts/{id}/messages", s.createCoachingMessage)
			r.Post("/coaching/contracts/{id}/rating", s.rateCoachingContract)
			r.Get("/me/coaching/contracts", s.listMyCoachingContracts)
			r.Get("/coach/requests", s.listCoachRequests)
			r.Get("/coach/requests/{id}", s.getCoachReviewRequest)
			r.Post("/coaching/requests/{id}/response", s.submitReviewResponse)
			r.Post("/coaching/requests/{id}/clarification", s.submitClarification)
			r.Post("/coaching/requests/{id}/clarify", s.clarifyReview)
			r.Post("/coaching/requests/{id}/rating", s.rateReview)
			r.Post("/coaching/requests/{id}/helpful", s.answerHelpful)
			r.Post("/coach/offers/{id}/accept", s.acceptReviewOffer)
			r.Post("/coach/offers/{id}/decline", s.declineReviewOffer)
			r.Get("/coaching/estimate", s.reviewEstimate)
			r.Post("/coaching/demo", s.joinCoachingDemo)
			r.Post("/coaching/requests", s.createReviewRequest)
			r.Post("/coaching/requests/{id}/checkout", s.checkoutReviewRequest)
			r.Get("/me/coaching/requests", s.listMyReviewRequests)
			r.Get("/coaching/requests/{id}", s.getReviewRequest)
			r.Post("/posts", s.createPost)
			r.Post("/posts/youtube", s.citeYouTube)
			r.Post("/posts/instagram", s.citeInstagram)
			r.Post("/uploads/video", s.signVideoUpload)
			r.Post("/uploads/image", s.signImageUpload)
			r.Post("/uploads/coaching-video", s.signCoachingVideoUpload)
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
			r.Get("/admin/inquiries", s.listInquiries)
			r.Post("/admin/posts/{id}/hide", s.hidePost)
			r.Post("/admin/comments/{id}/hide", s.hideComment)
			r.Post("/admin/injury-notes/{id}/hide", s.hideInjuryNote)
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
			r.Post("/admin/coaching/contracts/{id}/refund", s.adminRefundCoachingContract)
			r.Get("/admin/menu-contest", s.adminMenuContest)
			r.Put("/admin/menu-contest/finalists", s.setMenuContestFinalists)
		})
	})
	return r
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
		p.ThumbnailURL = s.cloud.ThumbnailURL(p.VideoPublicID)
		p.PlaybackURL = s.cloud.PlaybackURL(p.VideoPublicID)
	}
	if (p.Kind == "text" || p.Kind == "recruitment") && p.ImagePublicID != "" {
		p.ThumbnailURL = s.cloud.ImageURL(p.ImagePublicID)
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
