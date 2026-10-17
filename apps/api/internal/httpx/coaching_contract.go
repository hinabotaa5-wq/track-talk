package httpx

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// パーソナル指導の申し込みと、コーチの承諾・辞退。
// 利用者は申し込みで、コーチは承諾で、それぞれ「通報時に運営が読む」「連絡先の自動検出」に同意する。

// 契約開始時の同意の文面の版。文面（web の lib/coaching-plan.ts）を変えたら上げる。
const contractConsentVersion = 1

// コーチが返事をする期限。過ぎたら申し込みは閉じる。
const contractRespondWindow = 48 * time.Hour

type contractConsentReq struct {
	AgreeReport    bool `json:"agree_report"`
	AgreeDetection bool `json:"agree_detection"`
}

func (c contractConsentReq) fields() map[string]string {
	fields := map[string]string{}
	if !c.AgreeReport {
		fields["agree_report"] = "通報があったときに運営が読むことへの同意が必要です"
	}
	if !c.AgreeDetection {
		fields["agree_detection"] = "連絡先の自動検出への同意が必要です"
	}
	return fields
}

// getPublicCoachingPlan は、申し込む前に見るプランとコーチ。ログインしなくても見られる。
func (s *Server) getPublicCoachingPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	p, err := s.q.GetPublicCoachingPlan(r.Context(), db.GetPublicCoachingPlanParams{ID: id, PledgeVersion: coachPledgeVersion})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "このプランはいま申し込めません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	plan := coachingPlanJSON(db.CoachingPlan{
		ID: p.ID, CoachID: p.CoachID, Kind: p.Kind, Title: p.Title, Description: p.Description,
		DurationDays: p.DurationDays, PriceYen: p.PriceYen, Capacity: p.Capacity, Published: p.Published,
	})
	plan["full"] = p.ActiveCount >= int32(p.Capacity)
	JSON(w, http.StatusOK, map[string]any{
		"plan": plan,
		"coach": map[string]any{
			"id":        p.CoachID,
			"nickname":  p.CoachNickname,
			"photo_url": s.coachPhotoURL(p.CoachPhotoPublicID),
			"tier":      p.CoachTier,
			"events":    p.CoachEvents,
		},
	})
}

type coachingContractReq struct {
	PlanID        uuid.UUID `json:"plan_id"`
	Event         string    `json:"event"`
	Goal          string    `json:"goal"`
	CurrentRecord string    `json:"current_record"`
	Message       string    `json:"message"`
	contractConsentReq
}

func (s *Server) createCoachingContract(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if !me.CoachingDemo {
		Error(w, http.StatusForbidden, "forbidden", "パーソナル指導は準備中です")
		return
	}
	var req coachingContractReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	plan, err := s.q.GetPublicCoachingPlan(r.Context(), db.GetPublicCoachingPlanParams{ID: req.PlanID, PledgeVersion: coachPledgeVersion})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "このプランはいま申し込めません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	if plan.CoachID == me.ID {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "自分のプランには申し込めません")
		return
	}
	req.Goal = strings.TrimSpace(req.Goal)
	req.CurrentRecord = strings.TrimSpace(req.CurrentRecord)
	req.Message = strings.TrimSpace(req.Message)
	fields := req.fields()
	if !slices.Contains(plan.CoachEvents, req.Event) {
		fields["event"] = "このコーチが見る種目から選んでください"
	}
	if e := coachTextError("目標", req.Goal, 200); e != "" {
		fields["goal"] = e
	}
	if e := coachTextError("いまの記録", req.CurrentRecord, 100); e != "" {
		fields["current_record"] = e
	}
	if e := coachTextError("相談したいこと", req.Message, 1000); e != "" {
		fields["message"] = e
	}
	for key, text := range map[string]string{"goal": req.Goal, "current_record": req.CurrentRecord, "message": req.Message} {
		if fields[key] == "" && looksLikeContact(text) {
			fields[key] = "連絡先（URL・メールアドレス・SNS の ID・電話番号など）は書けません"
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if plan.ActiveCount >= int32(plan.Capacity) {
		Error(w, http.StatusConflict, "plan_full", "このプランはいま満員です")
		return
	}
	if me.BirthMonth == nil {
		Error(w, http.StatusUnprocessableEntity, "birth_month_required", "先に生まれた年月を登録してください")
		return
	}
	minor := isMinor(*me.BirthMonth, time.Now())
	if minor {
		// いまは添削と同じ保護者の同意を使う。契約ごとの保護者の同意は、保護者の参加と一緒に足す。
		consent, err := s.q.GetGuardianConsentByMinor(r.Context(), me.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			MapDBError(w, err)
			return
		}
		if err != nil || consent.Status != "consented" {
			Error(w, http.StatusUnprocessableEntity, "guardian_required", "18歳未満の人は、保護者の同意を得てから申し込めます")
			return
		}
	}

	var ct db.CoachingContract
	err = s.tx(r.Context(), func(q *db.Queries) error {
		var err error
		ct, err = q.CreateCoachingContract(r.Context(), db.CreateCoachingContractParams{
			ID:            uuid.New(),
			PlanID:        plan.ID,
			CoachID:       plan.CoachID,
			RequesterID:   me.ID,
			PlanKind:      plan.Kind,
			PlanTitle:     plan.Title,
			DurationDays:  plan.DurationDays,
			PriceYen:      plan.PriceYen,
			Event:         req.Event,
			Goal:          req.Goal,
			CurrentRecord: req.CurrentRecord,
			Message:       req.Message,
			Minor:         minor,
			RespondBy:     time.Now().Add(contractRespondWindow),
		})
		if err != nil {
			return err
		}
		return q.InsertContractConsent(r.Context(), db.InsertContractConsentParams{
			ContractID: ct.ID, Party: "requester", ProfileID: me.ID, Version: contractConsentVersion,
		})
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "このコーチには、返事待ちか期間中の申し込みがあります")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.pushQuietly(r.Context(), ct.CoachID, push.Message{
		Title: "パーソナル指導の申し込みが届きました",
		Body:  "「" + ct.PlanTitle + "」に申し込みがありました。48時間以内に返事をしてください",
		URL:   "/coach",
		Tag:   "contract-" + ct.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusCreated, map[string]any{"contract": s.contractJSON(ct)})
}

// contractJSON は、申し込みの中身。相手の名前などは呼ぶ側で足す。
func (s *Server) contractJSON(ct db.CoachingContract) map[string]any {
	return map[string]any{
		"id":             ct.ID,
		"plan_id":        ct.PlanID,
		"plan_kind":      ct.PlanKind,
		"plan_title":     ct.PlanTitle,
		"duration_days":  ct.DurationDays,
		"price_yen":      ct.PriceYen,
		"event":          ct.Event,
		"goal":           ct.Goal,
		"current_record": ct.CurrentRecord,
		"message":        ct.Message,
		"minor":          ct.Minor,
		"status":         ct.Status,
		"respond_by":     ct.RespondBy,
		"responded_at":   ct.RespondedAt,
		"starts_at":      ct.StartsAt,
		"ends_at":        ct.EndsAt,
		"created_at":     ct.CreatedAt,
	}
}

func (s *Server) listMyCoachingContracts(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListMyCoachingContracts(r.Context(), me.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		c := s.contractJSON(db.CoachingContract{
			ID: row.ID, PlanID: row.PlanID, CoachID: row.CoachID, RequesterID: row.RequesterID, PlanKind: row.PlanKind,
			PlanTitle: row.PlanTitle, DurationDays: row.DurationDays, PriceYen: row.PriceYen, Event: row.Event, Goal: row.Goal,
			CurrentRecord: row.CurrentRecord, Message: row.Message, Minor: row.Minor, Status: row.Status, RespondBy: row.RespondBy,
			RespondedAt: row.RespondedAt, StartsAt: row.StartsAt, EndsAt: row.EndsAt, CreatedAt: row.CreatedAt,
		})
		c["coach"] = map[string]any{"id": row.CoachID, "nickname": row.CoachNickname, "photo_url": s.coachPhotoURL(row.CoachPhotoPublicID)}
		out = append(out, c)
	}
	JSON(w, http.StatusOK, map[string]any{"contracts": out})
}

func (s *Server) listCoachContracts(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	rows, err := s.q.ListCoachContracts(r.Context(), coach.ProfileID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		c := s.contractJSON(db.CoachingContract{
			ID: row.ID, PlanID: row.PlanID, CoachID: row.CoachID, RequesterID: row.RequesterID, PlanKind: row.PlanKind,
			PlanTitle: row.PlanTitle, DurationDays: row.DurationDays, PriceYen: row.PriceYen, Event: row.Event, Goal: row.Goal,
			CurrentRecord: row.CurrentRecord, Message: row.Message, Minor: row.Minor, Status: row.Status, RespondBy: row.RespondBy,
			RespondedAt: row.RespondedAt, StartsAt: row.StartsAt, EndsAt: row.EndsAt, CreatedAt: row.CreatedAt,
		})
		// コーチには利用者のニックネームだけを見せる。
		c["requester"] = map[string]any{"nickname": row.RequesterNickname}
		out = append(out, c)
	}
	JSON(w, http.StatusOK, map[string]any{"contracts": out})
}

// getCoachingContract は、申し込んだ人か、そのコーチだけが見られる。
func (s *Server) getCoachingContract(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	row, err := s.q.GetCoachingContractDetail(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.RequesterID != me.ID && row.CoachID != me.ID) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	c := s.contractJSON(db.CoachingContract{
		ID: row.ID, PlanID: row.PlanID, CoachID: row.CoachID, RequesterID: row.RequesterID, PlanKind: row.PlanKind,
		PlanTitle: row.PlanTitle, DurationDays: row.DurationDays, PriceYen: row.PriceYen, Event: row.Event, Goal: row.Goal,
		CurrentRecord: row.CurrentRecord, Message: row.Message, Minor: row.Minor, Status: row.Status, RespondBy: row.RespondBy,
		RespondedAt: row.RespondedAt, StartsAt: row.StartsAt, EndsAt: row.EndsAt, CreatedAt: row.CreatedAt,
	})
	role := "requester"
	if row.CoachID == me.ID {
		role = "coach"
	}
	c["role"] = role
	c["coach"] = map[string]any{"id": row.CoachID, "nickname": row.CoachNickname, "photo_url": s.coachPhotoURL(row.CoachPhotoPublicID)}
	c["requester"] = map[string]any{"nickname": row.RequesterNickname}
	JSON(w, http.StatusOK, map[string]any{"contract": c})
}

func (s *Server) acceptCoachingContract(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req contractConsentReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if fields := req.fields(); len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	var ct db.CoachingContract
	err := s.tx(r.Context(), func(q *db.Queries) error {
		var err error
		ct, err = q.AcceptCoachingContract(r.Context(), db.AcceptCoachingContractParams{ID: id, CoachID: coach.ProfileID})
		if err != nil {
			return err
		}
		return q.InsertContractConsent(r.Context(), db.InsertContractConsentParams{
			ContractID: ct.ID, Party: "coach", ProfileID: coach.ProfileID, Version: contractConsentVersion,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		s.contractConflict(w, r, id, coach.ProfileID)
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.pushQuietly(r.Context(), ct.RequesterID, push.Message{
		Title: "パーソナル指導が始まりました",
		Body:  "コーチが「" + ct.PlanTitle + "」の申し込みを承諾しました",
		URL:   "/coaching/personal/" + ct.ID.String(),
		Tag:   "contract-" + ct.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"contract": s.contractJSON(ct)})
}

func (s *Server) declineCoachingContract(w http.ResponseWriter, r *http.Request) {
	coach, ok := s.approvedCoach(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ct, err := s.q.DeclineCoachingContract(r.Context(), db.DeclineCoachingContractParams{ID: id, CoachID: coach.ProfileID})
	if errors.Is(err, pgx.ErrNoRows) {
		s.contractConflict(w, r, id, coach.ProfileID)
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	s.pushQuietly(r.Context(), ct.RequesterID, push.Message{
		Title: "パーソナル指導の申し込みの結果",
		Body:  "今回はコーチの都合で受けられませんでした。ほかのコーチも探してみてください",
		URL:   "/coaching/personal/" + ct.ID.String(),
		Tag:   "contract-" + ct.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusOK, map[string]any{"contract": s.contractJSON(ct)})
}

// contractConflict は、承諾・辞退できなかった理由を返す。
func (s *Server) contractConflict(w http.ResponseWriter, r *http.Request, id, coachID uuid.UUID) {
	ct, err := s.q.GetCoachingContract(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && ct.CoachID != coachID) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	switch {
	case ct.Status != "applied":
		Error(w, http.StatusConflict, "conflict", "この申し込みは返事が済んでいます")
	case !ct.RespondBy.After(time.Now()):
		Error(w, http.StatusConflict, "conflict", "返事の期限を過ぎました")
	default:
		Error(w, http.StatusConflict, "plan_full", "このプランは同時に受ける人数の上限に届いています。プランの人数を増やすか、断ってください")
	}
}

func (s *Server) cancelCoachingContract(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ct, err := s.q.CancelCoachingContract(r.Context(), db.CancelCoachingContractParams{ID: id, RequesterID: me.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "conflict", "返事待ちの申し込みだけ取り下げられます")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"contract": s.contractJSON(ct)})
}

// closeCoachingContracts は、返事のないまま48時間たった申し込みと、期間が終わったものを閉じる。
func (s *Server) closeCoachingContracts(ctx context.Context) (expired, ended int, err error) {
	ex, err := s.q.ExpireCoachingApplications(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, ct := range ex {
		s.pushQuietly(ctx, ct.RequesterID, push.Message{
			Title: "パーソナル指導の申し込みの期限が過ぎました",
			Body:  "48時間のうちにコーチから返事がありませんでした。ほかのコーチも探してみてください",
			URL:   "/coaching/personal/" + ct.ID.String(),
			Tag:   "contract-" + ct.ID.String(),
		}, 24*time.Hour)
	}
	en, err := s.q.EndCoachingContracts(ctx)
	if err != nil {
		return len(ex), 0, err
	}
	for _, ct := range en {
		msg := push.Message{Title: "パーソナル指導の期間が終わりました", Body: "「" + ct.PlanTitle + "」の期間が終わりました", Tag: "contract-" + ct.ID.String()}
		msg.URL = "/coaching/personal/" + ct.ID.String()
		s.pushQuietly(ctx, ct.RequesterID, msg, 24*time.Hour)
		msg.URL = "/coach"
		s.pushQuietly(ctx, ct.CoachID, msg, 24*time.Hour)
	}
	return len(ex), len(en), nil
}

// tx は、fn を1つのトランザクションで行う。fn がエラーを返したら巻き戻す。
func (s *Server) tx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
