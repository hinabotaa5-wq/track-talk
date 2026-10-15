package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
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

	r.Route("/v1", func(r chi.Router) {
		r.Use(OptionalAuth(v))
		r.Get("/posts", s.listPosts)
		r.Get("/posts/{id}", s.getPost)
		r.Get("/posts/{id}/comments", s.listComments)
		r.Get("/injury/stats", s.injuryStats)
		r.Get("/injury/groups", s.listInjuryGroups)
		r.Get("/products", s.listProducts)
		r.Get("/products/{id}", s.getProduct)
		r.Get("/products/{id}/reviews", s.listProductReviews)
		r.Get("/push/public-key", s.pushPublicKey)
		// 保護者の同意のページ。ログインはいらず、リンクのトークンで本人を確かめる。
		r.Get("/guardian/{token}", s.getGuardian)
		r.Post("/guardian/{token}/consent", s.consentGuardian)
		r.Patch("/guardian/{token}", s.updateGuardian)
		r.Post("/guardian/{token}/decline", s.declineGuardian)
		r.Post("/guardian/{token}/revoke", s.revokeGuardian)

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
			r.Post("/admin/products", s.createProduct)
			r.Patch("/admin/products/{id}", s.updateProduct)
			r.Post("/admin/products/{id}/hide", s.hideProduct)
			r.Post("/admin/product-reviews/{id}/hide", s.hideProductReview)
		})
	})
	return r
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
