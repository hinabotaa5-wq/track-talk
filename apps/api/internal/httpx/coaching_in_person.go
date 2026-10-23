package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

func inPersonWritable(ct db.CoachingContract, completion bool) error {
	if ct.FulfillmentVersion != 2 || ct.MessagesPurgedAt != nil || ct.FulfillmentState != "open" {
		return &ticketRuleError{"in_person_closed", "この対面指導は変更できません"}
	}
	now := time.Now()
	if completion && ct.Status == "ended" && ct.EndsAt != nil && now.Before(ct.EndsAt.Add(48*time.Hour)) {
		return nil
	}
	if ct.Status != "active" || ct.EndsAt == nil || !now.Before(*ct.EndsAt) {
		return &ticketRuleError{"in_person_closed", "支援期間中だけ変更できます"}
	}
	return nil
}
func (s *Server) inPersonJSON(ctx context.Context, q *db.Queries, ct db.CoachingContract) (map[string]any, error) {
	usage, err := q.InPersonUsage(ctx, ct.ID)
	if err != nil {
		return nil, err
	}
	sessions, err := q.ListInPersonSessions(ctx, ct.ID)
	if err != nil {
		return nil, err
	}
	if sessions == nil {
		sessions = []db.CoachingInPersonSession{}
	}
	if ct.MessagesPurgedAt != nil {
		sessions = []db.CoachingInPersonSession{}
	}
	return map[string]any{"usage": usage, "sessions": sessions, "total_units": ct.SessionCount, "delivered_units": usage.Completed, "fulfillment_state": ct.FulfillmentState, "refund_amount_yen": ct.SupportRefundAmountYen, "venue_area": ct.VenueArea, "venue_name": ct.VenueName, "session_count": ct.SessionCount, "session_minutes": ct.SessionMinutes, "travel_fee_yen": ct.TravelFeeYen, "max_participants": ct.MaxParticipants}, nil
}
func (s *Server) getInPerson(w http.ResponseWriter, r *http.Request) {
	ct, role, ok := s.supportAccess(w, r)
	if !ok {
		return
	}
	if ct.FulfillmentVersion != 2 {
		Error(w, 404, "not_found", "この契約には対面指導はありません")
		return
	}
	out, err := s.inPersonJSON(r.Context(), s.q, ct)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out["role"] = role
	out["minor"] = ct.Minor
	JSON(w, 200, out)
}
func (s *Server) createInPersonSession(w http.ResponseWriter, r *http.Request) {
	initial, role, ok := s.supportAccess(w, r)
	if !ok {
		return
	}
	if role != "coach" {
		Error(w, 403, "forbidden", "候補は担当コーチが登録します")
		return
	}
	var req struct {
		RequestKey   uuid.UUID `json:"request_key"`
		StartsAt     time.Time `json:"starts_at"`
		VenueName    string    `json:"venue_name"`
		VenueArea    string    `json:"venue_area"`
		MeetingNote  string    `json:"meeting_note"`
		Participants int16     `json:"participants"`
	}
	if err := DecodeJSON(r, &req); err != nil || req.RequestKey == uuid.Nil || strings.TrimSpace(req.VenueName) == "" || strings.TrimSpace(req.VenueArea) == "" {
		FieldError(w, map[string]string{"venue_name": "日時・地域・会場を入力してください"})
		return
	}
	var sess db.CoachingInPersonSession
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockCallCalendar(r.Context()); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(r.Context(), initial.ID)
		if err != nil {
			return err
		}
		if err := inPersonWritable(ct, false); err != nil {
			return err
		}
		cp, err := q.GetCoachProfile(r.Context(), ct.CoachID)
		if err != nil {
			return err
		}
		if cp.Status != "approved" {
			return &ticketRuleError{"coach_unavailable", "承認済みの担当コーチだけが登録できます"}
		}
		old, err := q.GetInPersonSessionByKey(r.Context(), db.GetInPersonSessionByKeyParams{ContractID: ct.ID, RequestKey: req.RequestKey})
		if err == nil {
			if !old.StartsAt.Equal(req.StartsAt) || old.VenueName != req.VenueName {
				return &ticketRuleError{"session_conflict", "同じ操作の予約内容は変更できません"}
			}
			sess = old
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		participants := req.Participants
		if participants == 0 {
			participants = 1
		}
		if participants < 1 || participants > ct.MaxParticipants {
			return &ticketRuleError{"participants", "募集人数の範囲で登録してください"}
		}
		if len(req.MeetingNote) > 500 {
			return &ticketRuleError{"meeting_note", "集合方法は500文字以内で入力してください"}
		}
		end := req.StartsAt.Add(time.Duration(ct.SessionMinutes) * time.Minute)
		if !req.StartsAt.After(time.Now()) || ct.StartsAt == nil || req.StartsAt.Before(*ct.StartsAt) || end.After(*ct.EndsAt) {
			return &ticketRuleError{"session_outside_period", "対面は期間内の未来の日時で登録してください"}
		}
		usage, err := q.InPersonUsage(r.Context(), ct.ID)
		if err != nil {
			return err
		}
		if usage.Reserved+usage.Completed >= int32(ct.SessionCount) {
			return &ticketRuleError{"session_limit", "対面の残り回数がありません"}
		}
		overlap, err := q.InPersonOverlap(r.Context(), db.InPersonOverlapParams{CoachID: ct.CoachID, RequesterID: ct.RequesterID, StartsAt: req.StartsAt, EndsAt: end})
		if err != nil {
			return err
		}
		if overlap {
			return &ticketRuleError{"session_overlap", "コーチまたは利用者のほかの対面と日時が重なっています"}
		}
		sess, err = q.CreateInPersonSession(r.Context(), db.CreateInPersonSessionParams{ID: uuid.New(), ContractID: ct.ID, CoachID: ct.CoachID, RequesterID: ct.RequesterID, RequestKey: req.RequestKey, StartsAt: req.StartsAt, EndsAt: end, VenueName: strings.TrimSpace(req.VenueName), VenueArea: strings.TrimSpace(req.VenueArea), MeetingNote: req.MeetingNote, Participants: participants})
		if err != nil {
			return err
		}
		return supportAudit(r.Context(), q, ct, &ct.CoachID, "coach", "対面の候補日時を登録しました。専用の対面欄で日時・場所を確認して確定してください。")
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	s.notifyInPerson(r.Context(), initial)
	JSON(w, 200, map[string]any{"session": sess})
}
func (s *Server) notifyInPerson(ctx context.Context, ct db.CoachingContract) {
	msg := push.Message{Title: "対面指導が更新されました", Body: "契約の対面欄で日時・場所を確認してください", URL: "/coaching/personal/" + ct.ID.String(), Tag: "in-person-" + ct.ID.String()}
	s.notify(ctx, ct.RequesterID, notifyCoaching, msg, 24*time.Hour)
	s.notify(ctx, ct.CoachID, notifyCoaching, msg, 24*time.Hour)
}
func (s *Server) actInPersonSession(w http.ResponseWriter, r *http.Request) {
	initial, role, ok := s.supportAccess(w, r)
	if !ok {
		return
	}
	cid, ok := parseUUIDParam(w, r, "cid")
	if !ok {
		return
	}
	action := chi.URLParam(r, "action")
	var saved db.CoachingInPersonSession
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockCallCalendar(r.Context()); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(r.Context(), initial.ID)
		if err != nil {
			return err
		}
		if err := inPersonWritable(ct, action == "report-completed" || action == "confirm-completed" || action == "cancel"); err != nil {
			return err
		}
		c, err := q.GetInPersonSession(r.Context(), db.GetInPersonSessionParams{ID: cid, ContractID: ct.ID})
		if err != nil {
			return err
		}
		state := ""
		switch action {
		case "confirm":
			if role == "coach" || ct.Minor && role != "guardian" {
				return &ticketRuleError{"guardian_required", "利用者（未成年の場合は保護者）が日時・場所を確認してください"}
			}
			if c.Status != "proposed" || !c.StartsAt.After(time.Now()) {
				return &ticketRuleError{"session_closed", "この候補日時は確定できません"}
			}
			state = "confirmed"
		case "report-completed":
			if role != "coach" || c.Status != "confirmed" || time.Now().Before(c.EndsAt) {
				return &ticketRuleError{"session_not_finished", "終了後、担当コーチが実施を報告してください"}
			}
			state = "completion_pending"
		case "confirm-completed":
			if role == "coach" || ct.Minor && role != "guardian" || c.Status != "completion_pending" {
				return &ticketRuleError{"session_not_finished", "実施報告後、利用者（未成年は保護者）が確認してください"}
			}
			state = "completed"
		case "cancel":
			if c.Status == "completed" || c.Status == "cancelled" {
				return &ticketRuleError{"session_closed", "終了済みの対面は変更できません"}
			}
			state = "cancelled"
		default:
			return &ticketRuleError{"invalid_action", "操作が不正です"}
		}
		if role == "coach" {
			cp, err := q.GetCoachProfile(r.Context(), ct.CoachID)
			if err != nil {
				return err
			}
			if cp.Status != "approved" {
				return &ticketRuleError{"coach_unavailable", "このコーチは現在対応できません"}
			}
		}
		saved, err = q.ChangeInPersonSession(r.Context(), db.ChangeInPersonSessionParams{ID: cid, Status: state})
		if err != nil {
			return err
		}
		sender := &ct.RequesterID
		if role == "coach" {
			sender = &ct.CoachID
		}
		if role == "guardian" {
			sender = nil
		}
		labels := map[string]string{"confirmed": "対面の日時・場所を確定しました。", "completion_pending": "コーチが対面の実施を報告しました。参加者は実施を確認してください。", "completed": "参加者が対面の実施を確認しました。対面1回を提供済みにしました。", "cancelled": "対面を取り消しました。未提供の回数を戻しました。"}
		return supportAudit(r.Context(), q, ct, sender, role, labels[state])
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	s.notifyInPerson(r.Context(), initial)
	JSON(w, 200, map[string]any{"session": saved})
}
