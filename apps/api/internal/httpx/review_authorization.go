package httpx

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// An existing assignment does not retain access after coach approval is revoked.
// Read the current approval from the database on every coach review operation.
func (s *Server) requireApprovedReviewCoach(w http.ResponseWriter, r *http.Request) (db.Profile, bool) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return db.Profile{}, false
	}
	if !s.requireCurrentCoach(w, r, p.ID, "承認されたコーチだけが添削の依頼を操作できます") {
		return db.Profile{}, false
	}
	return p, true
}

// requireCurrentCoach は、コーチとして操作する人が、いまも承認済みで、二段階認証の条件を満たすかを確かめる。
// 申し込みや依頼の担当であっても、承認を取り消されたら読むことも送ることもできない。違えば 403 を返して false。
func (s *Server) requireCurrentCoach(w http.ResponseWriter, r *http.Request, id uuid.UUID, message string) bool {
	coach, err := s.q.GetCoachProfile(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && coach.Status != "approved") {
		Error(w, http.StatusForbidden, "forbidden", message)
		return false
	}
	if err != nil {
		MapDBError(w, err)
		return false
	}
	return s.requireStaffMFA(w, r)
}
