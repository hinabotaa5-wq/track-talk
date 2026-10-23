package httpx

import (
	"net/http"
	"strings"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

type inPersonPlanReq struct {
	VenueArea       *string `json:"venue_area"`
	VenueName       *string `json:"venue_name"`
	SessionCount    *int16  `json:"session_count"`
	SessionMinutes  *int16  `json:"session_minutes"`
	TravelFeeYen    *int32  `json:"travel_fee_yen"`
	MaxParticipants *int16  `json:"max_participants"`
	inPerson        db.SaveInPersonPlanParams
}

func (r *coachingPlanReq) inPersonFields(current db.CoachingPlan, published bool) map[string]string {
	p := db.SaveInPersonPlanParams{VenueArea: current.VenueArea, VenueName: current.VenueName, SessionCount: current.SessionCount, SessionMinutes: current.SessionMinutes, TravelFeeYen: current.TravelFeeYen, MaxParticipants: current.MaxParticipants}
	if p.SessionMinutes == 0 {
		p.SessionMinutes = 60
	}
	if p.MaxParticipants == 0 {
		p.MaxParticipants = 1
	}
	fields := map[string]string{}
	if r.VenueArea != nil {
		v := strings.TrimSpace(*r.VenueArea)
		p.VenueArea = nil
		if v != "" {
			p.VenueArea = &v
		}
	}
	if r.VenueName != nil {
		v := strings.TrimSpace(*r.VenueName)
		p.VenueName = nil
		if v != "" {
			p.VenueName = &v
		}
	}
	if r.SessionCount != nil {
		p.SessionCount = *r.SessionCount
	}
	if r.SessionMinutes != nil {
		p.SessionMinutes = *r.SessionMinutes
	}
	if r.TravelFeeYen != nil {
		p.TravelFeeYen = *r.TravelFeeYen
	}
	if r.MaxParticipants != nil {
		p.MaxParticipants = *r.MaxParticipants
	}
	if p.SessionCount < 0 || p.SessionCount > 12 {
		fields["session_count"] = "対面は0〜12回で設定してください"
	}
	if p.SessionMinutes != 30 && p.SessionMinutes != 60 && p.SessionMinutes != 90 && p.SessionMinutes != 120 {
		fields["session_minutes"] = "対面は30・60・90・120分から選んでください"
	}
	if p.TravelFeeYen < 0 || p.TravelFeeYen > 100000 {
		fields["travel_fee_yen"] = "交通費を確認してください"
	}
	if p.MaxParticipants < 1 || p.MaxParticipants > 20 {
		fields["max_participants"] = "募集人数は1〜20人で設定してください"
	}
	for _, f := range []struct {
		key string
		v   *string
	}{{"venue_area", p.VenueArea}, {"venue_name", p.VenueName}} {
		if f.v != nil {
			if e := coachPublicTextError("会場", *f.v, 100); e != "" {
				fields[f.key] = e
			}
		}
	}
	inPerson := r.ServiceType != nil && *r.ServiceType == "in_person"
	if inPerson {
		if r.Kind != "period" {
			fields["kind"] = "対面指導は期間プランを選んでください"
		}
		if published {
			if p.VenueArea == nil || p.VenueName == nil {
				fields["venue_name"] = "地域と会場を入力してください"
			}
			if p.SessionCount < 1 {
				fields["session_count"] = "対面は1〜12回で設定してください"
			}
		}
	} else if p.SessionCount != 0 {
		fields["session_count"] = "対面の回数は対面指導で設定できます"
	}
	r.inPerson = p
	return fields
}
func (s *Server) saveInPersonPlan(w http.ResponseWriter, r *http.Request, q *db.Queries, req coachingPlanReq, p db.CoachingPlan) (db.CoachingPlan, bool) {
	args := req.inPerson
	args.ID = p.ID
	updated, err := q.SaveInPersonPlan(r.Context(), args)
	if err != nil {
		MapDBError(w, err)
		return p, false
	}
	return updated, true
}
