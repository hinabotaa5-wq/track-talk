package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/hinata/track-talk/apps/api/internal/push"
	"github.com/jackc/pgx/v5"
)

// 継続プラン（期間プラン・1ヶ月プラン）のコーチ不在日の合意延長（本体 #898）。
//
//   - 対象は期間中（active）の期間プランと、無料相談の提案から成立した1ヶ月プラン。延長の事由は、コーチが契約上の役務を一時的に提供できない事情だけ。
//     大会の中止・延期を理由とする延長はしない。不在の理由は概要だけを記録する。
//   - 不在日は、コーチが契約チャットで事前に申告した対応不能期間の各暦日。復帰日は含めない。
//   - 延長は、選手（未成年は保護者も）とコーチの全員が明示的に同意したときだけ。無返信・既読のみは合意とみなさない。自動延長はしない。
//   - 延長は原則1回・累計7日以内。延長後の終了日が選手の目標大会日を過ぎるとき、復帰予定日が決まっていないとき、2回目の不在は提示しない。
//   - 日割りの分母（期間日数）は当初のまま。合意した不在日は経過日数に数えない（精算の計算は support_settlement.go・coaching_payment.go）。

const (
	absenceMaxDays = 7 // 運営ポリシーとしての累計の上限（法定の上限ではない）。
	absenceMaxChar = 200
)

// absenceKind は、延長の対象かどうかと、その種類。support は1ヶ月プラン、period は期間プラン。対象外は空。
func absenceKind(ct db.CoachingContract) string {
	if ct.Status != "active" || ct.StartsAt == nil || ct.EndsAt == nil || ct.SupportEndCause != nil || ct.FulfillmentState != "open" {
		return ""
	}
	if ct.StartOnFirstVideo && ct.ClockStartedAt == nil {
		return "" // 最初の動画から数えるプランで、まだ期間が始まっていない。
	}
	switch {
	case supportConsultContract(ct):
		return "support"
	case ct.FulfillmentVersion == 0 && ct.PlanKind == "period" && ct.ServiceType == nil:
		return "period"
	}
	return ""
}

// absenceOffer は、申告された不在期間から、延長後の終了日を決める。提示できないときは理由を返す。
// goal は選手の目標大会日（1ヶ月プランのみ。決まっていなければ nil）。
func absenceOffer(ct db.CoachingContract, from, ret, now time.Time, goal *time.Time, declaring bool) (time.Time, *ticketRuleError) {
	if absenceKind(ct) == "" {
		return time.Time{}, &ticketRuleError{"absence_not_eligible", "期間中の期間プラン・1ヶ月プランだけ、不在日の延長を申し出られます"}
	}
	if ct.AbsenceDays > 0 {
		return time.Time{}, &ticketRuleError{"absence_second", "不在日の延長は原則1回までです。2回目の不在は延長の対象になりません"}
	}
	today := supportDay(now)
	if declaring && from.Before(today) {
		return time.Time{}, &ticketRuleError{"absence_period_invalid", "不在の開始日は今日以降にしてください（事前の申告だけが対象です）"}
	}
	days := int(ret.Sub(from).Hours() / 24)
	if days < 1 {
		return time.Time{}, &ticketRuleError{"absence_period_invalid", "復帰予定日は不在の開始日より後にしてください"}
	}
	if days > absenceMaxDays {
		return time.Time{}, &ticketRuleError{"absence_limit", fmt.Sprintf("延長は累計%d日までです（運営の方針）", absenceMaxDays)}
	}
	start, lastDay := supportDay(*ct.StartsAt), supportDay(ct.EndsAt.Add(-time.Second))
	if from.Before(start) || ret.After(lastDay.AddDate(0, 0, 1)) {
		return time.Time{}, &ticketRuleError{"absence_period_invalid", "不在の期間は、契約の期間内にしてください"}
	}
	newEnds := ct.EndsAt.Add(time.Duration(days) * 24 * time.Hour)
	if goal != nil && supportDay(newEnds.Add(-time.Second)).After(calendarDate(*goal)) {
		return time.Time{}, &ticketRuleError{"absence_past_goal", "延長すると終了日が目標の大会の日を過ぎるため、延長は提示できません"}
	}
	return newEnds, nil
}

// absenceGoalDate は、1ヶ月プランの選手の目標大会日（無料相談の回答）。決まっていなければ nil。
func (s *Server) absenceGoalDate(ctx context.Context, q *db.Queries, ct db.CoachingContract) (*time.Time, error) {
	if absenceKind(ct) != "support" {
		return nil, nil
	}
	c, err := q.GetSupportConsultByContract(ctx, &ct.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var a consultAnswers
	if json.Unmarshal(c.Answers, &a) != nil || a.CompetitionDate == "" {
		return nil, nil
	}
	d, err := time.Parse(time.DateOnly, a.CompetitionDate)
	if err != nil {
		return nil, nil
	}
	return &d, nil
}

// absenceRefundEstimate は、いま終了した場合の返金概算額（今日を経過扱い、未経過分の日割り全額返金）。
func (s *Server) absenceRefundEstimate(ctx context.Context, q *db.Queries, ct db.CoachingContract, now time.Time) int32 {
	if absenceKind(ct) == "support" {
		return supportUnprovidedAsOf(ct, supportDay(now))
	}
	amount := ct.PriceYen
	if pay, err := q.GetPaidCoachingPayment(ctx, &ct.ID); err == nil {
		amount = pay.AmountYen
	}
	return remainingRefund(ct, db.CoachingPayment{AmountYen: amount}, now)
}

func jpDate(t time.Time) string { return t.In(jst.Zone).Format("2006年1月2日") }

func absenceJSON(e db.CoachingAbsenceExtension, now time.Time, viewer string) map[string]any {
	status := e.Status
	if status == "declared" && !supportDay(now).Before(calendarDate(e.ReturnOn)) {
		status = "lapsed" // 復帰予定日までに合意がそろわなかった（記録の更新は次の操作のとき）。
	}
	out := map[string]any{
		"id": e.ID, "status": status, "absent_from": e.AbsentFrom.Format(time.DateOnly), "return_on": e.ReturnOn.Format(time.DateOnly),
		"absent_days": e.AbsentDays, "reason": e.Reason, "minor": e.Minor, "price_yen": e.PriceYen,
		"original_ends_at": e.OriginalEndsAt, "proposed_ends_at": e.ProposedEndsAt, "refund_estimate_yen": e.RefundEstimateYen,
		"declared_at": e.DeclaredAt, "max_total_days": absenceMaxDays,
		"player_consented_at": e.PlayerConsentedAt, "guardian_consented_at": e.GuardianConsentedAt, "coach_consented_at": e.CoachConsentedAt,
		"agreed_at": e.AgreedAt, "agreed_original_ends_at": e.AgreedOriginalEndsAt, "agreed_new_ends_at": e.AgreedNewEndsAt,
	}
	if viewer == "coach" && len(e.CoachTerms) > 0 {
		var terms any
		_ = json.Unmarshal(e.CoachTerms, &terms)
		out["coach_terms"] = terms
	}
	return out
}

type absenceDeclareReq struct {
	AbsentFrom string `json:"absent_from"`
	ReturnOn   string `json:"return_on"`
	Reason     string `json:"reason"`
}

// declareCoachAbsence は、コーチが不在期間（開始日・復帰予定日・理由の概要）を申告する。契約チャットに記録し、選手へ延長案を示す。
func (s *Server) declareCoachAbsence(w http.ResponseWriter, r *http.Request) {
	me, ct, role, ok := s.contractParty(w, r)
	if !ok {
		return
	}
	if role != "coach" {
		Error(w, http.StatusForbidden, "forbidden", "不在日の申告は担当コーチだけができます")
		return
	}
	var req absenceDeclareReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	fields := map[string]string{}
	from, errFrom := time.Parse(time.DateOnly, req.AbsentFrom)
	if errFrom != nil {
		fields["absent_from"] = "不在の開始日を選んでください"
	}
	ret, errRet := time.Parse(time.DateOnly, req.ReturnOn)
	if errRet != nil {
		fields["return_on"] = "復帰予定日を選んでください（決まっていないときは延長を申し出られません）"
	}
	if n := len([]rune(req.Reason)); n == 0 || n > absenceMaxChar {
		fields["reason"] = fmt.Sprintf("理由の概要を%d字以内で書いてください（病名・診断などの詳細は書かないでください）", absenceMaxChar)
	} else if looksLikeContact(req.Reason) {
		fields["reason"] = "連絡先（URL・メールアドレス・SNS の ID・電話番号など）は書けません"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	now := time.Now()
	var out db.CoachingAbsenceExtension
	err := s.tx(r.Context(), func(q *db.Queries) error {
		cur, err := q.LockConsultationContract(r.Context(), ct.ID)
		if err != nil {
			return err
		}
		if cur.CoachID != me.ID {
			return pgx.ErrNoRows
		}
		// 復帰予定日を過ぎても合意がそろわなかった申告は閉じる。
		exts, err := q.ListAbsenceExtensions(r.Context(), &cur.ID)
		if err != nil {
			return err
		}
		for _, e := range exts {
			if e.Status != "declared" {
				continue
			}
			if supportDay(now).Before(calendarDate(e.ReturnOn)) {
				return &ticketRuleError{"absence_open", "合意待ちの不在の申告があります。先にそれを取り下げるか、選手の返事を待ってください"}
			}
			if _, err := q.CloseAbsenceExtension(r.Context(), db.CloseAbsenceExtensionParams{ID: e.ID, Status: "lapsed"}); err != nil {
				return err
			}
		}
		goal, err := s.absenceGoalDate(r.Context(), q, cur)
		if err != nil {
			return err
		}
		newEnds, rule := absenceOffer(cur, from, ret, now, goal, true)
		if rule != nil {
			return rule
		}
		out, err = q.InsertAbsenceExtension(r.Context(), db.InsertAbsenceExtensionParams{
			ContractID: &cur.ID, CoachID: &cur.CoachID, RequesterID: &cur.RequesterID,
			AbsentFrom: from, ReturnOn: ret, AbsentDays: int32(ret.Sub(from).Hours() / 24), Reason: req.Reason, Minor: cur.Minor, PriceYen: cur.PriceYen,
			OriginalEndsAt: *cur.EndsAt, ProposedEndsAt: newEnds, RefundEstimateYen: s.absenceRefundEstimate(r.Context(), q, cur, now),
		})
		if err != nil {
			return err
		}
		return s.recordAbsenceMessage(r.Context(), q, cur, &me.ID, "coach", "absence_declared", fmt.Sprintf(
			"【不在日の申告】%s〜%s（復帰は%s・%d日間）。理由: %s\n延長する場合の新しい終了日は%sです（追加料金はなく、価格と内容は変わりません）。延長は選手とコーチ双方の同意があったときだけ成立します。",
			jpDate(from), jpDate(ret.AddDate(0, 0, -1)), jpDate(ret), out.AbsentDays, req.Reason, jpDate(newEnds.Add(-time.Second))))
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	s.notify(r.Context(), ct.RequesterID, notifyImportant, push.Message{
		Title: "コーチの不在日の延長について、ご確認ください", URL: "/coaching/personal/" + ct.ID.String(), Tag: "absence-" + out.ID.String(),
	}, 24*time.Hour)
	JSON(w, http.StatusCreated, map[string]any{"absence": absenceJSON(out, now, "coach")})
}

// recordAbsenceMessage は、不在の申告・合意の記録を契約チャットに残す。コーチの返信とは数えない（system_kind）。
func (s *Server) recordAbsenceMessage(ctx context.Context, q *db.Queries, ct db.CoachingContract, sender *uuid.UUID, role, kind, body string) error {
	k := kind
	_, err := q.CreateSystemCoachingMessage(ctx, db.CreateSystemCoachingMessageParams{
		ID: uuid.New(), ContractID: ct.ID, SenderID: sender, SenderRole: role, Body: body, SystemKind: &k,
	})
	return err
}

// listContractAbsences は、契約の当事者が、不在日の延長の申告と合意の履歴を見る。契約チャットが閲覧できなくなった後も見られる。
func (s *Server) listContractAbsences(w http.ResponseWriter, r *http.Request) {
	_, ct, role, ok := s.contractParty(w, r)
	if !ok {
		return
	}
	s.writeAbsences(w, r, ct, role)
}

func (s *Server) writeAbsences(w http.ResponseWriter, r *http.Request, ct db.CoachingContract, viewer string) {
	exts, err := s.q.ListAbsenceExtensions(r.Context(), &ct.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	now := time.Now()
	list := make([]map[string]any, 0, len(exts))
	for _, e := range exts {
		list = append(list, absenceJSON(e, now, viewer))
	}
	JSON(w, http.StatusOK, map[string]any{
		"absences": list, "eligible": absenceKind(ct) != "" && ct.AbsenceDays == 0, "max_total_days": absenceMaxDays,
		"ends_at": ct.EndsAt, "absence_days": ct.AbsenceDays,
		"absence_from": dateString(ct.AbsenceFrom), "absence_return_on": dateString(ct.AbsenceReturnOn),
	})
}

type absenceConsentReq struct {
	Agree bool `json:"agree"`
}

// consentAbsence は、選手が延長に同意する（agree=true）か、延長しないと答える（agree=false）。無返信・既読では何も起きない。
func (s *Server) consentAbsence(w http.ResponseWriter, r *http.Request) {
	me, ct, role, ok := s.contractParty(w, r)
	if !ok {
		return
	}
	if role != "requester" {
		Error(w, http.StatusForbidden, "forbidden", "延長に同意できるのは契約した選手だけです")
		return
	}
	s.actAbsence(w, r, ct, "requester", &me.ID)
}

// consentGuardianAbsence は、未成年選手の契約で、保護者が延長に同意する（または延長しないと答える）。
func (s *Server) consentGuardianAbsence(w http.ResponseWriter, r *http.Request) {
	ct, ok := s.guardianContract(w, r)
	if !ok {
		return
	}
	s.actAbsence(w, r, ct, "guardian", nil)
}

// coachConsentAbsence は、コーチが変更後の期間を確認して同意する（agree=false は取り下げ）。
func (s *Server) coachConsentAbsence(w http.ResponseWriter, r *http.Request) {
	me, ct, role, ok := s.contractParty(w, r)
	if !ok {
		return
	}
	if role != "coach" {
		Error(w, http.StatusForbidden, "forbidden", "担当コーチだけが同意できます")
		return
	}
	s.actAbsence(w, r, ct, "coach", &me.ID)
}

func (s *Server) listGuardianAbsences(w http.ResponseWriter, r *http.Request) {
	ct, ok := s.guardianContract(w, r)
	if !ok {
		return
	}
	s.writeAbsences(w, r, ct, "guardian")
}

// actAbsence は、同意または不同意の1回の操作。全員の同意がそろったときだけ、同じトランザクションで終了日を更新する。
func (s *Server) actAbsence(w http.ResponseWriter, r *http.Request, ct db.CoachingContract, actor string, actorID *uuid.UUID) {
	aid, ok := parseUUIDParam(w, r, "aid")
	if !ok {
		return
	}
	var req absenceConsentReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	now := time.Now()
	var out db.CoachingAbsenceExtension
	var agreed bool
	err := s.tx(r.Context(), func(q *db.Queries) error {
		cur, err := q.LockConsultationContract(r.Context(), ct.ID)
		if err != nil {
			return err
		}
		e, err := q.GetAbsenceExtensionForUpdate(r.Context(), db.GetAbsenceExtensionForUpdateParams{ID: aid, ContractID: &cur.ID})
		if err != nil {
			return err
		}
		out = e
		if e.Status != "declared" {
			return &ticketRuleError{"absence_closed", "この不在日の延長は、すでに終わっています"}
		}
		if !supportDay(now).Before(calendarDate(e.ReturnOn)) {
			// 復帰予定日を過ぎた。合意がそろわなかったので、終了日は変えない。
			if out, err = q.CloseAbsenceExtension(r.Context(), db.CloseAbsenceExtensionParams{ID: e.ID, Status: "lapsed"}); err != nil {
				return err
			}
			return nil
		}
		if actor == "guardian" && !e.Minor {
			return pgx.ErrNoRows
		}
		if !req.Agree {
			status, body := "declined", "選手は、不在日の延長をしないと答えました。契約の終了日は元のままです。継続するか、いつでも終了するかを選べます。"
			switch actor {
			case "coach":
				status, body = "withdrawn", "コーチは、不在日の延長の申し出を取り下げました。契約の終了日は元のままです。"
			case "guardian":
				body = "保護者は、不在日の延長をしないと答えました。契約の終了日は元のままです。"
			}
			if out, err = q.CloseAbsenceExtension(r.Context(), db.CloseAbsenceExtensionParams{ID: e.ID, Status: status}); err != nil {
				return err
			}
			return s.recordAbsenceMessage(r.Context(), q, cur, actorID, actorRole(actor), "absence_closed", body)
		}
		switch actor {
		case "requester":
			out, err = q.SetAbsencePlayerConsent(r.Context(), db.SetAbsencePlayerConsentParams{ID: e.ID, PlayerConsentedBy: actorID})
		case "guardian":
			out, err = q.SetAbsenceGuardianConsent(r.Context(), e.ID)
		case "coach":
			out, err = q.SetAbsenceCoachConsent(r.Context(), db.SetAbsenceCoachConsentParams{ID: e.ID, CoachConsentedBy: actorID})
		}
		if err != nil {
			return err
		}
		complete := out.PlayerConsentedAt != nil && out.CoachConsentedAt != nil && (!out.Minor || out.GuardianConsentedAt != nil)
		if !complete {
			return nil
		}
		return s.applyAbsenceAgreement(r.Context(), q, cur, out, now, &out, &agreed)
	})
	var rule *ticketRuleError
	if errors.As(err, &rule) {
		// 提示の条件が変わっていて成立させられない（目標大会日を過ぎる等）。終了日は変えない。
		writeTicketRuleError(w, err)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err != nil {
		writeCheckoutError(w, err)
		return
	}
	if agreed {
		msg := push.Message{Title: "不在日の延長に合意し、契約の終了日を変更しました", URL: "/coaching/personal/" + ct.ID.String(), Tag: "absence-" + out.ID.String()}
		s.notify(r.Context(), ct.RequesterID, notifyImportant, msg, 24*time.Hour)
		msg.URL = "/coach/support-contracts/" + ct.ID.String()
		s.notify(r.Context(), ct.CoachID, notifyImportant, msg, 24*time.Hour)
	}
	JSON(w, http.StatusOK, map[string]any{"absence": absenceJSON(out, now, actorRole(actor)), "agreed": agreed})
}

func actorRole(actor string) string {
	if actor == "coach" || actor == "guardian" {
		return actor
	}
	return "requester"
}

// applyAbsenceAgreement は、全員の同意がそろったときに、条件を確かめ直して終了日を更新し、変更履歴と取引条件の再明示を残す。
// 条件が変わっていて提示できなくなっていたら、終了日は変えずにエラーを返す。
func (s *Server) applyAbsenceAgreement(ctx context.Context, q *db.Queries, cur db.CoachingContract, e db.CoachingAbsenceExtension, now time.Time, result *db.CoachingAbsenceExtension, agreed *bool) error {
	goal, err := s.absenceGoalDate(ctx, q, cur)
	if err != nil {
		return err
	}
	// 申告のときと同じ条件を、合意の時点の契約で確かめ直す（すでに始まった不在でも、申告済みなら同意を受ける）。
	newEnds, rule := absenceOffer(cur, e.AbsentFrom, e.ReturnOn, now, goal, false)
	if rule != nil {
		return rule
	}
	original := *cur.EndsAt
	updated, err := q.ApplyContractAbsence(ctx, db.ApplyContractAbsenceParams{
		Days: e.AbsentDays, AbsentFrom: &e.AbsentFrom, ReturnOn: &e.ReturnOn, ID: cur.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &ticketRuleError{"absence_not_eligible", "この契約は、いま延長できません"}
		}
		return err
	}
	terms, err := json.Marshal(absenceCoachTerms(updated, e, original, newEnds))
	if err != nil {
		return err
	}
	done, err := q.AgreeAbsenceExtension(ctx, db.AgreeAbsenceExtensionParams{ID: e.ID, AgreedOriginalEndsAt: &original, AgreedNewEndsAt: &newEnds, CoachTerms: terms})
	if err != nil {
		return err
	}
	*result, *agreed = done, true
	return s.recordAbsenceMessage(ctx, q, cur, nil, "coach", "absence_agreed", fmt.Sprintf(
		"【不在日の延長が成立しました】不在%d日間（%s〜%s）。契約の終了日は%sから%sに変わりました。追加料金はなく、価格と内容は変わりません。選手の同意%s、コーチの同意があり、変更の内容は契約の画面でも確認できます。",
		e.AbsentDays, jpDate(e.AbsentFrom), jpDate(e.ReturnOn.AddDate(0, 0, -1)), jpDate(original.Add(-time.Second)), jpDate(newEnds.Add(-time.Second)),
		map[bool]string{true: "・保護者の同意", false: ""}[e.Minor]))
}

// absenceCoachTerms は、コーチへ再明示する、変更後の提供期日を含む取引条件（フリーランス法第3条に対応する形）。
func absenceCoachTerms(ct db.CoachingContract, e db.CoachingAbsenceExtension, original, newEnds time.Time) map[string]any {
	terms := map[string]any{
		"title":            "取引条件の変更の明示（コーチ不在日の合意延長）",
		"contract_id":      ct.ID.String(),
		"changed_on":       supportDay(time.Now()).Format(time.DateOnly),
		"absent_from":      e.AbsentFrom.Format(time.DateOnly),
		"return_on":        e.ReturnOn.Format(time.DateOnly),
		"absent_days":      e.AbsentDays,
		"original_end_on":  supportDay(original.Add(-time.Second)).Format(time.DateOnly),
		"new_end_on":       supportDay(newEnds.Add(-time.Second)).Format(time.DateOnly),
		"price_yen":        ct.PriceYen,
		"coach_share_yen":  supportCoachShare(ct.PriceYen),
		"service":          "サポート内容と価格は変わりません。提供する期日の終わりが、不在日数分だけ後ろ倒しになります。",
		"pay_calculation":  "報酬の日割りの分母（期間日数）は、当初の契約期間の日数のまま変わりません。合意した不在日は提供対象日から除外し、経過日数に数えません（契約終了時に、経過日数の日割りの75%で確定します）。",
		"advance":          "月次の仮払いは、延長を理由に留保・後ろ倒しはしません。",
		"payment_due":      supportPayoutDayText(),
		"consent":          "選手（未成年の場合は保護者を含む）とコーチの全員が明示的に同意したため、終了日を変更しました。",
		"agreed_extension": "延長は原則1回・累計7日以内です。",
	}
	return terms
}
