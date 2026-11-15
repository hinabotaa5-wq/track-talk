package httpx

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/jackc/pgx/v5"
)

func (c identityReader) configured() bool {
	return c.url != "" && c.token != ""
}

// coachIdentityStatus は、コーチ本人の身元登録の有無だけを返す。氏名・住所などの中身は一切返さない。
func (s *Server) coachIdentityStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	coach, err := s.q.GetCoachProfile(ctx, p.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && coach.Status != "approved") {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// applies は「受付中かどうか」で決める。担当サービスだけ・下書きのプランだけでは対象にしない。
	applies := false
	if st, err := s.q.GetSupportConsultSettings(ctx, coach.ProfileID); err == nil {
		applies = st.Accepting
	} else if !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	if !applies {
		plans, err := s.q.ListMyCoachingPlans(ctx, coach.ProfileID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		for _, pl := range plans {
			if !pl.Published || pl.ConsultTemplate {
				continue
			}
			if pl.ServiceType != nil && (*pl.ServiceType == "competition_support" || *pl.ServiceType == "in_person") {
				applies = true
				break
			}
		}
	}
	var registered *bool
	if s.identity.configured() {
		ok, err := s.identity.get(ctx, "/internal/registration/"+url.PathEscape(coach.ProfileID.String()), nil)
		if err == nil {
			v := ok
			registered = &v
		}
	}
	JSON(w, http.StatusOK, map[string]any{"registered": registered, "applies": applies})
}
