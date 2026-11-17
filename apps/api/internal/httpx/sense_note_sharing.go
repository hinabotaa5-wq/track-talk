package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5"
)

// 継続プラン（period）の契約で、選手の感覚メモを担当コーチと共有する。
// 共有の有無は sense_note_contract_exclusions の行の有無で表す（行が無ければ共有中）。
// 既存の本人用経路（/v1/me/sense-notes）と summary の仕組みは変えない。

// contractSenseNote は、共有の一覧に出す1件。既存の senseNoteDTO に shared を足した形。
type contractSenseNote struct {
	senseNoteDTO
	// Shared は、選手がチェックを付けている（コーチに見せている）か。
	Shared bool `json:"shared"`
}

// sharingWindowStart は、契約開始日（Asia/Tokyo の日付）の1ヶ月前の同日。
// 同日が存在しない月（例：開始日 3月31日 → 2月）は、その月の末日にする。
// Go の AddDate(0, -1, 0) の素の正規化（3月31日 → 3月3日）は使わない。
func sharingWindowStart(startsAt time.Time) time.Time {
	y, m, d := startsAt.In(jst.Zone).Date()
	prev := m - 1
	py := y
	if prev < time.January {
		prev = time.December
		py--
	}
	last := time.Date(py, prev+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > last {
		d = last
	}
	return time.Date(py, prev, d, 0, 0, 0, 0, time.UTC)
}

// contractSenseClosed は、契約が終わって共有の一覧を閉じたか（ended・refunded）。
func contractSenseClosed(status string) bool {
	return status == "ended" || status == "refunded"
}

func (s *Server) listContractSenseNotes(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ct.RequesterID != me.ID && ct.CoachID != me.ID) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	role := "requester"
	if ct.CoachID == me.ID {
		role = "coach"
		if !s.requireCurrentCoach(w, r, me.ID, contractCoachForbidden) {
			return
		}
	}
	now := s.now()
	// 終わった契約は、選手・コーチとも本文を返さない（チャットと同じ形）。
	if contractSenseClosed(ct.Status) {
		JSON(w, http.StatusOK, map[string]any{"notes": []contractSenseNote{}, "view_closed": true})
		return
	}
	// 開くのは period の active だけ。開始前・それ以外は両者とも空。
	if ct.PlanKind != "period" || ct.Status != "active" || ct.StartsAt == nil {
		JSON(w, http.StatusOK, map[string]any{"notes": []contractSenseNote{}, "view_closed": false})
		return
	}
	windowStart := sharingWindowStart(*ct.StartsAt)
	today := dateOnly(now.In(jst.Zone))
	// コーチが読めるのは、期間が終わる前の間だけ。過ぎたら本文を返さない。
	if role == "coach" && (ct.EndsAt == nil || !now.Before(*ct.EndsAt)) {
		JSON(w, http.StatusOK, map[string]any{"notes": []contractSenseNote{}, "view_closed": true})
		return
	}
	rows, err := s.q.ListContractSenseNotes(r.Context(), db.ListContractSenseNotesParams{
		ContractID: id, AuthorID: ct.RequesterID, WindowStart: windowStart, Today: today,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	notes := make([]contractSenseNote, 0, len(rows))
	for _, row := range rows {
		if role == "coach" && row.Excluded {
			continue
		}
		notes = append(notes, contractSenseNote{
			senseNoteDTO: senseNoteFrom(row.ID, row.Body, row.Event, row.Day, row.CreatedAt, row.UpdatedAt),
			Shared:       !row.Excluded,
		})
	}
	JSON(w, http.StatusOK, map[string]any{"notes": notes, "view_closed": false})
}

type contractSenseShareReq struct {
	Shared *bool `json:"shared"`
}

func (s *Server) shareContractSenseNote(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	noteID, ok := parseUUIDParam(w, r, "note_id")
	if !ok {
		return
	}
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	// requester 本人だけ。コーチ・第三者は契約の存在自体を隠す（404）。
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ct.RequesterID != me.ID) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	var req contractSenseShareReq
	if err := DecodeJSON(r, &req); err != nil || req.Shared == nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	// 管理が開くのは period の active だけ。それ以外は 409。
	if ct.PlanKind != "period" || ct.Status != "active" || ct.StartsAt == nil {
		Error(w, http.StatusConflict, "contract_not_active", "期間中だけ共有を変えられます")
		return
	}
	note, err := s.q.GetContractSenseNote(r.Context(), db.GetContractSenseNoteParams{ID: noteID, AuthorID: me.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	day := dateOnly(note.Day.In(jst.Zone))
	windowStart := sharingWindowStart(*ct.StartsAt)
	today := dateOnly(s.now().In(jst.Zone))
	if day.Before(windowStart) || day.After(today) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if *req.Shared {
		err = s.q.DeleteSenseNoteContractExclusion(r.Context(), db.DeleteSenseNoteContractExclusionParams{
			ContractID: id, SenseNoteID: noteID,
		})
	} else {
		err = s.q.InsertSenseNoteContractExclusion(r.Context(), db.InsertSenseNoteContractExclusionParams{
			ContractID: id, SenseNoteID: noteID,
		})
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"shared": *req.Shared})
}

// keepRewriteExclusions は、感覚メモを消す直前に、そのメモに除外があった契約を日付（日本時間）とともに残す。
// メモを消すと除外の行も一緒に消えるため、同じ日に書き直したメモへ引き継ぐにはここで残しておく必要がある。
// 残すのは今日の分のメモだけ（書き直せるのは今日の分だけ）で、本文は残さない。
// その日の古い記録は先に捨て、消す時点の除外の状態だけを正にする。
func keepRewriteExclusions(ctx context.Context, q *db.Queries, authorID, noteID uuid.UUID, noteDay, now time.Time) error {
	today := dateOnly(now.In(jst.Zone))
	if err := q.ClearSenseNoteRewriteExclusions(ctx, db.ClearSenseNoteRewriteExclusionsParams{
		AuthorID: authorID, NoteDay: dateOnly(noteDay), Today: today,
	}); err != nil {
		return err
	}
	return q.SaveSenseNoteRewriteExclusions(ctx, db.SaveSenseNoteRewriteExclusionsParams{
		NoteID: noteID, AuthorID: authorID, Today: today,
	})
}

// inheritRewriteExclusions は、作った感覚メモに、同じ日に消したメモの除外を契約ごとに引き継ぎ、
// その選手の記録を捨てる。除外がなかった契約・別の日の記録は引き継がない。
func inheritRewriteExclusions(ctx context.Context, q *db.Queries, authorID, noteID uuid.UUID, day time.Time) error {
	if err := q.InheritSenseNoteRewriteExclusions(ctx, db.InheritSenseNoteRewriteExclusionsParams{
		NoteID: noteID, AuthorID: authorID, Day: day,
	}); err != nil {
		return err
	}
	return q.DeleteSenseNoteRewriteExclusions(ctx, authorID)
}
