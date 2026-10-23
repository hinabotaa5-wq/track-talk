package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

func (s *Server) reportConsultation(w http.ResponseWriter, r *http.Request, reporter, id uuid.UUID, reason, detail string, guardian bool) {
	err := s.tx(r.Context(), func(q *db.Queries) error {
		initial, err := q.GetConsultation(r.Context(), id)
		if err != nil {
			return err
		}
		root, _ := consultationParent(initial)
		if err := q.LockConsultationRoot(r.Context(), root); err != nil {
			return err
		}
		c, err := q.LockConsultation(r.Context(), id)
		if err != nil {
			return err
		}
		owner := c.RequesterID != nil && *c.RequesterID == reporter
		coach := c.CoachID != nil && *c.CoachID == reporter && !guardian
		if c.PurgedAt != nil || c.Status == "draft" || c.Status == "awaiting_payment" || (!owner && !coach) || guardian && !c.Minor || owner && !guardian && c.AnsweredAt == nil {
			return pgx.ErrNoRows
		}
		if guardian {
			_, err = q.CreateGuardianConsultationReport(r.Context(), db.CreateGuardianConsultationReportParams{ID: uuid.New(), ReporterID: reporter, TargetID: id, Reason: reason, Detail: strings.TrimSpace(detail)})
		} else {
			_, err = q.CreateReport(r.Context(), db.CreateReportParams{ID: uuid.New(), ReporterID: reporter, TargetType: "coaching_consultation", TargetID: id, Reason: reason, Detail: strings.TrimSpace(detail)})
		}
		return err
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	c, err := s.q.GetConsultation(r.Context(), id)
	if err == nil && c.Minor {
		s.notifyAdminsMinorReport(r.Context())
	}
	JSON(w, 201, map[string]any{"reported": true})
}
func (s *Server) reportGuardianConsultation(w http.ResponseWriter, r *http.Request) {
	c, role, ok := s.consultationParty(w, r)
	if !ok {
		return
	}
	if role != "guardian" {
		Error(w, 403, "forbidden", "保護者の操作です")
		return
	}
	var req struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, 400, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	if e := ReportReason(req.Reason); e != "" {
		fields["reason"] = e
	}
	if e := ReportDetail(req.Detail); e != "" {
		fields["detail"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	s.reportConsultation(w, r, *c.RequesterID, c.ID, req.Reason, req.Detail, true)
}
func (s *Server) adminConsultations(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	rows, err := s.q.AdminConsultations(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.AdminConsultationsRow{}
	}
	JSON(w, 200, map[string]any{"consultations": rows})
}
func (s *Server) consultationReported(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return id, uuid.Nil, false
	}
	report, err := uuid.Parse(r.URL.Query().Get("report"))
	if err != nil {
		Error(w, 403, "report_required", "通報があるときだけ開けます")
		return id, report, false
	}
	exists, err := s.q.ConsultationHasReport(r.Context(), db.ConsultationHasReportParams{ID: report, TargetID: id})
	if err != nil {
		MapDBError(w, err)
		return id, report, false
	}
	if !exists {
		Error(w, 403, "report_required", "この相談への通報を指定してください")
		return id, report, false
	}
	return id, report, true
}
func (s *Server) adminOpenConsultation(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	id, report, ok := s.consultationReported(w, r)
	if !ok {
		return
	}
	if err := s.q.LogConsultationAccess(r.Context(), db.LogConsultationAccessParams{ID: uuid.New(), AdminID: admin.ID, ConsultationID: id, ReportID: report}); err != nil {
		MapDBError(w, err)
		return
	}
	c, err := s.q.GetConsultation(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 非表示の内容も通報対応でのみ確認できる。
	hidden := c.HiddenAt != nil
	c.HiddenAt = nil
	out := s.consultationJSON(c)
	out["hidden"] = hidden
	JSON(w, 200, map[string]any{"consultation": out})
}
func (s *Server) adminHideConsultation(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, _, ok := s.consultationReported(w, r)
	if !ok {
		return
	}
	err := s.tx(r.Context(), func(q *db.Queries) error {
		initial, err := q.GetConsultation(r.Context(), id)
		if err != nil {
			return err
		}
		root, _ := consultationParent(initial)
		if err := q.LockConsultationRoot(r.Context(), root); err != nil {
			return err
		}
		c, err := q.LockConsultation(r.Context(), id)
		if err != nil {
			return err
		}
		switch c.Status {
		case "draft", "awaiting_payment", "offered", "in_review":
			c, err = q.CloseConsultation(r.Context(), db.CloseConsultationParams{ID: id, Status: "cancelled"})
			if err != nil {
				return err
			}
			if err := s.releaseConsultationFunding(r.Context(), q, c); err != nil {
				return err
			}
		}
		return q.HideConsultation(r.Context(), id)
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	JSON(w, 200, map[string]any{"hidden": true})
}
func (s *Server) adminRefundConsultation(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	err := s.tx(r.Context(), func(q *db.Queries) error {
		c, err := q.LockConsultation(r.Context(), id)
		if err != nil {
			return err
		}
		if c.Funding != "cash" || c.PaymentState != "refund_pending" || (c.Status != "expired" && c.Status != "declined" && c.Status != "cancelled") {
			return &ticketRuleError{"refund_unavailable", "未回答で終了した相談の返金だけ再試行できます"}
		}
		rows, err := q.ListConsultationRefunds(r.Context(), id)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Status != "failed" {
				return nil
			}
		}
		_, err = q.CreateConsultationRefund(r.Context(), db.CreateConsultationRefundParams{ID: uuid.New(), ConsultationID: id})
		return err
	})
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	JSON(w, 202, map[string]any{"accepted": true})
}
func (s *Server) purgeConsultations(ctx context.Context) (int, error) {
	rows, err := s.q.ListConsultationsToPurge(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, initial := range rows {
		purged := false
		err = s.tx(ctx, func(q *db.Queries) error {
			root, _ := consultationParent(initial)
			if err := q.LockConsultationRoot(ctx, root); err != nil {
				return err
			}
			c, err := q.LockConsultation(ctx, initial.ID)
			if err != nil {
				return err
			}
			eligible, err := q.ListConsultationsToPurge(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, row := range eligible {
				if row.ID == c.ID {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
			videos := []string{}
			if c.VideoPublicID != nil {
				videos = append(videos, *c.VideoPublicID)
			}
			if c.AnswerVideoPublicID != nil {
				videos = append(videos, *c.AnswerVideoPublicID)
			}
			if err := s.cloud.DeleteCoachingVideos(ctx, videos); err != nil {
				return err
			}
			if err := q.PurgeConsultation(ctx, c.ID); err != nil {
				return err
			}
			purged = true
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return n, err
			}
			return n, err
		}
		if purged {
			n++
		}
	}
	return n, nil
}

func (s *Server) listGuardianConsultationReviews(w http.ResponseWriter, r *http.Request) {
	minor, ok := s.guardianMinor(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListGuardianConsultationReviews(r.Context(), minor)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.ListGuardianConsultationReviewsRow{}
	}
	JSON(w, 200, map[string]any{"reviews": rows})
}
