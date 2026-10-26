package httpx

import (
	"errors"
	"net/http"

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
	coach, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && coach.Status != "approved") {
		Error(w, http.StatusForbidden, "forbidden", "承認されたコーチだけが添削の依頼を操作できます")
		return db.Profile{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.Profile{}, false
	}
	if !s.requireStaffMFA(w, r) {
		return db.Profile{}, false
	}
	return p, true
}
