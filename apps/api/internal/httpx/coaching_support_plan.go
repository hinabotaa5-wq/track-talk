package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

type supportPlanReq struct {
	CompetitionName  *string `json:"competition_name"`
	CompetitionDate  *string `json:"competition_date"`
	SupportStartDate *string `json:"support_start_date"`
	CallCount        *int16  `json:"call_count"`
	CallMinutes      *int16  `json:"call_minutes"`
	support          db.SaveSupportPlanParams
}

func dateString(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format("2006-01-02")
}
func (r *coachingPlanReq) supportFields(current db.CoachingPlan, published bool) map[string]string {
	p := db.SaveSupportPlanParams{CompetitionName: current.CompetitionName, CompetitionDate: current.CompetitionDate, SupportStartDate: current.SupportStartDate, CallCount: current.CallCount, CallMinutes: current.CallMinutes}
	if p.CallMinutes == 0 {
		p.CallMinutes = 30
	}
	fields := map[string]string{}
	if r.CompetitionName != nil {
		n := strings.TrimSpace(*r.CompetitionName)
		p.CompetitionName = nil
		if n != "" {
			p.CompetitionName = &n
		}
	}
	for _, f := range []struct {
		key    string
		raw    *string
		target **time.Time
	}{{"competition_date", r.CompetitionDate, &p.CompetitionDate}, {"support_start_date", r.SupportStartDate, &p.SupportStartDate}} {
		if f.raw != nil {
			*f.target = nil
			if *f.raw != "" {
				d, e := time.Parse("2006-01-02", *f.raw)
				if e != nil {
					fields[f.key] = "日付を選んでください"
				} else {
					*f.target = &d
				}
			}
		}
	}
	if r.CallCount != nil {
		p.CallCount = *r.CallCount
	}
	if r.CallMinutes != nil {
		p.CallMinutes = *r.CallMinutes
	}
	if p.CallCount < 0 || p.CallCount > 12 {
		fields["call_count"] = "通話は0〜12回で設定してください"
	}
	if p.CallMinutes != 15 && p.CallMinutes != 30 && p.CallMinutes != 45 && p.CallMinutes != 60 {
		fields["call_minutes"] = "通話は15・30・45・60分から選んでください"
	}
	if p.CompetitionName != nil {
		if e := coachPublicTextError("大会名", *p.CompetitionName, 100); e != "" {
			fields["competition_name"] = e
		}
	}
	competition := r.ServiceType != nil && *r.ServiceType == "competition_support"
	if competition {
		if r.Kind != "period" {
			fields["kind"] = "大会サポートは期間プランを選んでください"
		}
		if r.StartOnFirstVideo {
			fields["start_on_first_video"] = "大会サポートは設定した日程で提供します"
		}
		if p.CompetitionDate != nil && p.SupportStartDate != nil {
			days := int(p.CompetitionDate.Sub(*p.SupportStartDate).Hours()/24) + 1
			if days < 1 || days > 90 {
				fields["competition_date"] = "支援期間は1〜90日にしてください"
			} else {
				r.DurationDays = &days
			}
		}
		if published {
			if p.CompetitionName == nil {
				fields["competition_name"] = "大会名を入力してください"
			}
			if p.CompetitionDate == nil || p.CompetitionDate.Before(time.Now().UTC().Truncate(24*time.Hour)) {
				fields["competition_date"] = "これから開催する大会の日付を選んでください"
			}
			if p.SupportStartDate == nil {
				fields["support_start_date"] = "支援開始日を選んでください"
			}
			if r.VideoReviews == nil || *r.VideoReviews < 1 {
				fields["video_reviews"] = "大会サポートの動画添削は1〜30回で設定してください"
			}
		}
	} else if p.CallCount != 0 {
		fields["call_count"] = "通話は大会サポートで設定できます"
	}
	r.support = p
	return fields
}
func (s *Server) saveSupportPlan(w http.ResponseWriter, r *http.Request, q *db.Queries, req coachingPlanReq, p db.CoachingPlan) (db.CoachingPlan, bool) {
	args := req.support
	args.ID = p.ID
	updated, err := q.SaveSupportPlan(r.Context(), args)
	if err != nil {
		MapDBError(w, err)
		return p, false
	}
	return updated, true
}
