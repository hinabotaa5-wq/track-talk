package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
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
			r.Get("/me/saves", s.listSaves)
			r.Get("/me/inquiries", s.listMyInquiries)
			r.Post("/me/inquiries", s.createInquiry)
			r.Get("/me/notifications", s.listNotifications)
			r.Post("/me/notifications/read", s.markAllNotificationsRead)
			r.Post("/me/notifications/{id}/read", s.markNotificationRead)
			r.Post("/posts", s.createPost)
			r.Post("/posts/youtube", s.citeYouTube)
			r.Post("/uploads/video", s.signVideoUpload)
			r.Post("/uploads/image", s.signImageUpload)
			r.Patch("/posts/{id}", s.updatePost)
			r.Delete("/posts/{id}", s.deletePost)
			r.Post("/posts/{id}/likes", s.likePost)
			r.Delete("/posts/{id}/likes", s.unlikePost)
			r.Post("/posts/{id}/saves", s.savePost)
			r.Delete("/posts/{id}/saves", s.unsavePost)
			r.Post("/posts/{id}/comments", s.createComment)
			r.Post("/posts/{id}/injury/updates", s.createInjuryUpdate)
			r.Delete("/posts/{id}/injury/updates/{updateID}", s.deleteInjuryUpdate)
			r.Post("/posts/{id}/practice/applications", s.applyPractice)
			r.Post("/posts/{id}/practice/applications/{applicationID}/accept", s.acceptPractice)
			r.Post("/posts/{id}/practice/applications/{applicationID}/decline", s.declinePractice)
			r.Post("/posts/{id}/practice/applications/{applicationID}/rating", s.ratePractice)
			r.Post("/posts/{id}/practice/dissolve", s.dissolvePractice)
			r.Patch("/comments/{id}", s.updateComment)
			r.Delete("/comments/{id}", s.deleteComment)
			r.Post("/reports", s.createReport)
			r.Get("/admin/reports", s.listReports)
			r.Get("/admin/inquiries", s.listInquiries)
			r.Post("/admin/posts/{id}/hide", s.hidePost)
			r.Post("/admin/comments/{id}/hide", s.hideComment)
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
