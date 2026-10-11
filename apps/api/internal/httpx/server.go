package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/hinata/track-talk/apps/api/internal/auth"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	cfg   config.Config
	pool  *pgxpool.Pool
	q     *db.Queries
	auth  *auth.Verifier
	limit *RateLimiter
}

func New(cfg config.Config, pool *pgxpool.Pool, v *auth.Verifier) http.Handler {
	s := &Server{cfg: cfg, pool: pool, q: db.New(pool), auth: v, limit: NewRateLimiter()}
	r := chi.NewRouter()
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
		r.Get("/categories", s.listCategories)
		r.Get("/competitions", s.listCompetitions)
		r.Get("/competitions/{id}", s.getCompetition)
		r.Get("/competitions/{id}/posts", s.listCompetitionPosts)
		r.Get("/posts", s.listPosts)
		r.Get("/posts/{id}", s.getPost)
		r.Get("/posts/{id}/comments", s.listComments)
		r.Get("/users/{id}", s.getUser)
		r.Get("/users/{id}/posts", s.listUserPosts)

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(v))
			r.Get("/me", s.me)
			r.Put("/me/profile", s.upsertProfile)
			r.Post("/posts", s.createPost)
			r.Patch("/posts/{id}", s.updatePost)
			r.Delete("/posts/{id}", s.deletePost)
			r.Post("/posts/{id}/likes", s.likePost)
			r.Delete("/posts/{id}/likes", s.unlikePost)
			r.Post("/posts/{id}/comments", s.createComment)
			r.Patch("/comments/{id}", s.updateComment)
			r.Delete("/comments/{id}", s.deleteComment)
			r.Post("/reports", s.createReport)
			r.Get("/admin/reports", s.listReports)
			r.Post("/admin/posts/{id}/hide", s.hidePost)
			r.Post("/admin/comments/{id}/hide", s.hideComment)
		})
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		Error(w, http.StatusServiceUnavailable, "unhealthy", "データベースに接続できません")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
