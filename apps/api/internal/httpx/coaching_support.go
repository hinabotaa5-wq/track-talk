package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

var meetPath = regexp.MustCompile(`^/[a-z]{3}-[a-z]{4}-[a-z]{3}$`)
var zoomPath = regexp.MustCompile(`^/j/[0-9]{8,15}$`)

// 通話リンクは専用欄だけに保存する。HTTPSの既知の会議ホスト以外や、任意の転送URLは受け付けない。
func validCallURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || len(raw) > 2000 {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "meet.google.com":
		return meetPath.MatchString(u.Path) && u.RawQuery == ""
	case host == "zoom.us" || strings.HasSuffix(host, ".zoom.us"):
		if !zoomPath.MatchString(u.Path) {
			return false
		}
		for key := range u.Query() {
			if key != "pwd" {
				return false
			}
		}
		return true
	case host == "teams.microsoft.com" || host == "teams.live.com":
		return strings.HasPrefix(u.Path, "/l/meetup-join/") && len(u.Path) > 20
	}
	return false
}
func (s *Server) supportAccess(w http.ResponseWriter, r *http.Request) (db.CoachingContract, string, bool) {
	if chi.URLParam(r, "token") != "" {
		ct, ok := s.guardianContract(w, r)
		return ct, "guardian", ok
	}
	_, ct, role, ok := s.contractParty(w, r)
	return ct, role, ok
}
func supportWritable(ct db.CoachingContract, completion bool) error {
	if ct.FulfillmentVersion != 1 || ct.MessagesPurgedAt != nil || ct.FulfillmentState != "open" {
		return &ticketRuleError{"support_closed", "この提供内容は変更できません"}
	}
	now := time.Now()
	if completion && ct.Status == "ended" && ct.EndsAt != nil && now.Before(ct.EndsAt.Add(48*time.Hour)) {
		return nil
	}
	if ct.Status != "active" || ct.EndsAt == nil || !now.Before(*ct.EndsAt) {
		return &ticketRuleError{"support_closed", "支援期間中だけ変更できます"}
	}
	return nil
}
func (s *Server) supportJSON(ctx context.Context, q *db.Queries, ct db.CoachingContract) (map[string]any, error) {
	usage, err := q.SupportUsage(ctx, ct.ID)
	if err != nil {
		return nil, err
	}
	calls, err := q.ListSupportCalls(ctx, ct.ID)
	if err != nil {
		return nil, err
	}
	videos, err := q.ListSupportVideos(ctx, ct.ID)
	if err != nil {
		return nil, err
	}
	if calls == nil {
		calls = []db.CoachingCall{}
	}
	if videos == nil {
		videos = []db.ListSupportVideosRow{}
	}
	if ct.MessagesPurgedAt != nil {
		calls = []db.CoachingCall{}
		videos = []db.ListSupportVideosRow{}
	}
	total := int32(ct.CallCount)
	if ct.VideoReviews != nil {
		total += int32(*ct.VideoReviews)
	}
	return map[string]any{"usage": usage, "calls": calls, "videos": videos, "total_units": total, "delivered_units": usage.VideoAnswered + usage.CallsCompleted, "fulfillment_state": ct.FulfillmentState, "refund_amount_yen": ct.SupportRefundAmountYen, "competition_name": ct.CompetitionName, "competition_date": dateString(ct.CompetitionDate), "support_start_date": dateString(ct.SupportStartDate), "call_count": ct.CallCount, "call_minutes": ct.CallMinutes, "video_reviews": ct.VideoReviews}, nil
}
func (s *Server) getCoachingSupport(w http.ResponseWriter, r *http.Request) {
	ct, role, ok := s.supportAccess(w, r)
	if !ok {
		return
	}
	if ct.FulfillmentVersion != 1 {
		Error(w, 404, "not_found", "この契約には大会サポートはありません")
		return
	}
	out, err := s.supportJSON(r.Context(), s.q, ct)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out["role"] = role
	out["minor"] = ct.Minor
	JSON(w, 200, out)
}
func (s *Server) createSupportCall(w http.ResponseWriter, r *http.Request) {
	initial, role, ok := s.supportAccess(w, r)
	if !ok {
		return
	}
	if role != "coach" {
		Error(w, 403, "forbidden", "候補は担当コーチが登録します")
		return
	}
	var req struct {
		RequestKey uuid.UUID `json:"request_key"`
		StartsAt   time.Time `json:"starts_at"`
		JoinURL    string    `json:"join_url"`
	}
	if err := DecodeJSON(r, &req); err != nil || req.RequestKey == uuid.Nil || !validCallURL(req.JoinURL) {
		FieldError(w, map[string]string{"join_url": "Google Meet・Zoom・Teamsの会議参加リンクと日時を入力してください"})
		return
	}
	var call db.CoachingCall
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockCallCalendar(r.Context()); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(r.Context(), initial.ID)
		if err != nil {
			return err
		}
		if err := supportWritable(ct, false); err != nil {
			return err
		}
		cp, err := q.GetCoachProfile(r.Context(), ct.CoachID)
		if err != nil {
			return err
		}
		if cp.Status != "approved" {
			return &ticketRuleError{"coach_unavailable", "承認済みの担当コーチだけが登録できます"}
		}
		old, err := q.GetSupportCallByKey(r.Context(), db.GetSupportCallByKeyParams{ContractID: ct.ID, RequestKey: req.RequestKey})
		if err == nil {
			if !old.StartsAt.Equal(req.StartsAt) || old.JoinUrl != req.JoinURL {
				return &ticketRuleError{"call_conflict", "同じ操作の予約内容は変更できません"}
			}
			call = old
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		end := req.StartsAt.Add(time.Duration(ct.CallMinutes) * time.Minute)
		if !req.StartsAt.After(time.Now()) || ct.StartsAt == nil || req.StartsAt.Before(*ct.StartsAt) || end.After(*ct.EndsAt) {
			return &ticketRuleError{"call_outside_period", "通話は支援期間内の未来の日時で登録してください"}
		}
		usage, err := q.SupportUsage(r.Context(), ct.ID)
		if err != nil {
			return err
		}
		if usage.CallsReserved+usage.CallsCompleted >= int32(ct.CallCount) {
			return &ticketRuleError{"call_limit", "通話の残り回数がありません"}
		}
		overlap, err := q.SupportCallOverlap(r.Context(), db.SupportCallOverlapParams{CoachID: ct.CoachID, RequesterID: ct.RequesterID, StartsAt: req.StartsAt, EndsAt: end})
		if err != nil {
			return err
		}
		if overlap {
			return &ticketRuleError{"call_overlap", "コーチまたは利用者のほかの通話と日時が重なっています"}
		}
		call, err = q.CreateSupportCall(r.Context(), db.CreateSupportCallParams{ID: uuid.New(), ContractID: ct.ID, CoachID: ct.CoachID, RequesterID: ct.RequesterID, RequestKey: req.RequestKey, StartsAt: req.StartsAt, EndsAt: end, JoinUrl: req.JoinURL})
		if err != nil {
			return err
		}
		return supportAudit(r.Context(), q, ct, &ct.CoachID, "coach", "通話の候補日時を登録しました。専用の通話欄で日時・長さを確認して確定してください。")
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	s.notifySupport(r.Context(), initial)
	JSON(w, 200, map[string]any{"call": call})
}
func supportAudit(ctx context.Context, q *db.Queries, ct db.CoachingContract, sender *uuid.UUID, role, body string) error {
	_, err := q.CreateCoachingMessage(ctx, db.CreateCoachingMessageParams{ID: uuid.New(), ContractID: ct.ID, SenderID: sender, SenderRole: role, Body: body})
	return err
}
func (s *Server) notifySupport(ctx context.Context, ct db.CoachingContract) {
	msg := push.Message{Title: "大会サポートの通話が更新されました", Body: "契約の通話欄で日時・実施状況を確認してください", URL: "/coaching/personal/" + ct.ID.String(), Tag: "support-" + ct.ID.String()}
	s.notify(ctx, ct.RequesterID, notifyCoaching, msg, 24*time.Hour)
	s.notify(ctx, ct.CoachID, notifyCoaching, msg, 24*time.Hour)
}
func (s *Server) actSupportCall(w http.ResponseWriter, r *http.Request) {
	initial, role, ok := s.supportAccess(w, r)
	if !ok {
		return
	}
	cid, ok := parseUUIDParam(w, r, "cid")
	if !ok {
		return
	}
	action := chi.URLParam(r, "action")
	var saved db.CoachingCall
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockCallCalendar(r.Context()); err != nil {
			return err
		}
		ct, err := q.LockConsultationContract(r.Context(), initial.ID)
		if err != nil {
			return err
		}
		if err := supportWritable(ct, action == "report-completed" || action == "confirm-completed" || action == "cancel"); err != nil {
			return err
		}
		c, err := q.GetSupportCall(r.Context(), db.GetSupportCallParams{ID: cid, ContractID: ct.ID})
		if err != nil {
			return err
		}
		state := ""
		switch action {
		case "confirm":
			if role == "coach" || ct.Minor && role != "guardian" {
				return &ticketRuleError{"guardian_required", "利用者（未成年の場合は保護者）が日時を確認してください"}
			}
			if c.Status != "proposed" || !c.StartsAt.After(time.Now()) {
				return &ticketRuleError{"call_closed", "この候補日時は確定できません"}
			}
			state = "confirmed"
		case "report-completed":
			if role != "coach" || c.Status != "confirmed" || time.Now().Before(c.EndsAt) {
				return &ticketRuleError{"call_not_finished", "通話終了後、担当コーチが実施を報告してください"}
			}
			state = "completion_pending"
		case "confirm-completed":
			if role == "coach" || ct.Minor && role != "guardian" || c.Status != "completion_pending" {
				return &ticketRuleError{"call_not_finished", "実施報告後、利用者（未成年は保護者）が確認してください"}
			}
			state = "completed"
		case "cancel":
			if c.Status == "completed" || c.Status == "cancelled" {
				return &ticketRuleError{"call_closed", "終了済みの通話は変更できません"}
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
		saved, err = q.ChangeSupportCall(r.Context(), db.ChangeSupportCallParams{ID: cid, Status: state})
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
		labels := map[string]string{"confirmed": "通話の日時を確定しました。", "completion_pending": "コーチが通話の実施を報告しました。参加者は実施を確認してください。", "completed": "参加者が通話の実施を確認しました。通話1回を提供済みにしました。", "cancelled": "通話を取り消しました。未提供の回数を戻しました。日時変更は新しい候補を登録してください。"}
		return supportAudit(r.Context(), q, ct, sender, role, labels[state])
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	s.notifySupport(r.Context(), initial)
	JSON(w, 200, map[string]any{"call": saved})
}
