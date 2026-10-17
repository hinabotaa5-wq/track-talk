package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 運営のパーソナル指導の画面。契約の一覧には会話の中身を出さない。
// 会話は、その契約のやりとり欄に通報があるときだけ、通報を指定して開ける。開くたびに記録を残す。

// notifyAdminsMinorReport は、18歳未満が絡むやりとり欄の通報を、運営にすぐ知らせる。中身は載せない。
func (s *Server) notifyAdminsMinorReport(ctx context.Context) {
	ids, err := s.q.ListAdminProfileIDs(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		s.pushQuietly(ctx, id, push.Message{
			Title: "18歳未満のパーソナル指導に通報",
			Body:  "管理者画面のパーソナル指導から確認してください",
			URL:   "/admin",
			Tag:   "coaching-minor-report",
		}, 24*time.Hour)
	}
}

func (s *Server) adminListCoachingContracts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rows, err := s.q.AdminListCoachingContracts(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	blocked, err := s.q.ListCoachingBlockedAccounts(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.AdminListCoachingContractsRow{}
	}
	if blocked == nil {
		blocked = []db.ListCoachingBlockedAccountsRow{}
	}
	JSON(w, http.StatusOK, map[string]any{"contracts": rows, "blocked_accounts": blocked})
}

// adminOpenCoachingContract は、通報のある契約の会話を開く。report にその契約のやりとり欄への通報の ID が要る。
func (s *Server) adminOpenCoachingContract(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	reportID, err := uuid.Parse(r.URL.Query().Get("report"))
	if err != nil {
		Error(w, http.StatusForbidden, "report_required", "通報があるときだけ開けます")
		return
	}
	reported, err := s.q.CoachingReportOnContract(r.Context(), db.CoachingReportOnContractParams{ReportID: reportID, ContractID: id})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !reported {
		Error(w, http.StatusForbidden, "report_required", "通報があるときだけ開けます")
		return
	}
	row, err := s.q.GetCoachingContractDetail(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 記録が残せないときは開かない。
	if err := s.q.InsertCoachingAccessLog(r.Context(), db.InsertCoachingAccessLogParams{
		ID: uuid.New(), AdminID: admin.ID, ContractID: id, ReportID: reportID,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	msgs, err := s.q.ListCoachingMessages(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	logs, err := s.q.ListCoachingAccessLogs(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	c := s.contractJSON(db.CoachingContract{
		ID: row.ID, PlanID: row.PlanID, CoachID: row.CoachID, RequesterID: row.RequesterID, PlanKind: row.PlanKind,
		PlanTitle: row.PlanTitle, DurationDays: row.DurationDays, PriceYen: row.PriceYen, Event: row.Event, Goal: row.Goal,
		CurrentRecord: row.CurrentRecord, Message: row.Message, Minor: row.Minor, Status: row.Status, RespondBy: row.RespondBy,
		RespondedAt: row.RespondedAt, StartsAt: row.StartsAt, EndsAt: row.EndsAt, CreatedAt: row.CreatedAt,
		StartBy: row.StartBy, SuspendedAt: row.SuspendedAt, PaymentRequired: row.PaymentRequired,
	})
	c["coach"] = map[string]any{"id": row.CoachID, "nickname": row.CoachNickname, "photo_url": s.coachPhotoURL(row.CoachPhotoPublicID)}
	c["requester"] = map[string]any{"nickname": row.RequesterNickname}
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, s.coachingMessageJSON(m, uuid.Nil, "admin"))
	}
	if logs == nil {
		logs = []db.ListCoachingAccessLogsRow{}
	}
	JSON(w, http.StatusOK, map[string]any{"contract": c, "messages": out, "access_logs": logs})
}

func (s *Server) adminSuspendCoachingContract(w http.ResponseWriter, r *http.Request) {
	s.adminSetCoachingContract(w, r, true)
}

func (s *Server) adminResumeCoachingContract(w http.ResponseWriter, r *http.Request) {
	s.adminSetCoachingContract(w, r, false)
}

// adminSetCoachingContract は、期間中のやりとりを止めるか、止めていたものを再開する。両方に知らせる。
func (s *Server) adminSetCoachingContract(w http.ResponseWriter, r *http.Request, suspend bool) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var ct db.CoachingContract
	var err error
	msg := push.Message{Title: "パーソナル指導のやりとりを再開しました", Body: "止めていた日数の分、期間を延ばしました"}
	if suspend {
		ct, err = s.q.SuspendCoachingContract(r.Context(), id)
		msg = push.Message{Title: "パーソナル指導のやりとりを止めました", Body: "運営が確認しています。終わるまで送れません"}
	} else {
		ct, err = s.q.ResumeCoachingContract(r.Context(), id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if suspend {
			Error(w, http.StatusConflict, "conflict", "期間中のものだけ止められます")
		} else {
			Error(w, http.StatusConflict, "conflict", "止めているものだけ再開できます")
		}
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	msg.Tag = "contract-" + ct.ID.String()
	msg.URL = "/coaching/personal/" + ct.ID.String()
	s.pushQuietly(r.Context(), ct.RequesterID, msg, 24*time.Hour)
	s.pushQuietly(r.Context(), ct.CoachID, msg, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"contract": s.contractJSON(ct)})
}
