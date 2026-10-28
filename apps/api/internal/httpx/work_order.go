package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5"
)

// 取引条件の通知(フリーランス保護法3条)。運営者がフォーム添削とその追加相談をコーチに委託するたびに、
// その案件の9項目を、引き受ける前の打診の画面に出し、引き受けたら書き留める(coach_work_orders)。
// 弁護士の回答で「利用規約への同意だけでは足りず、案件ごとの明示が要る」とされたため。
// サポートプランはコーチが販売者なので、ここには入れない(要否は弁護士の回答 Q9 を待つ)。
// 支払期日の書き方(繰り越しがあるときに1つの日付に決まらない点)と、画面での明示で足りるかは回答待ち(Q6・Q7)。

// 通知の文面の版。文面を変えたら上げる。書き留めた通知は、そのときの版のまま残る。
const workOrderTermsVersion = 1

// 運営者(委託する側)の名前。web の lib/operator.ts と揃える。
const operatorName = "中窪日向"

type workOrderItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type workOrderTerms struct {
	Items       []workOrderItem `json:"items"`
	AmountYen   int32           `json:"amount_yen"`
	DeliverBy   time.Time       `json:"deliver_by"`
	PaymentDue  time.Time       `json:"payment_due"`
	CarriedDue  time.Time       `json:"carried_due"`
	ConsignedAt time.Time       `json:"consigned_at"`
}

func jstDate(t time.Time) string {
	return t.In(jst.Zone).Format("2006年1月2日")
}

func jstDateTime(t time.Time) string {
	return t.In(jst.Zone).Format("2006年1月2日 15:04")
}

// payoutDates は、deliverBy までに確定した報酬を支払う日と、繰り越したときの支払日(どちらも日本時間の15日)。
func payoutDates(deliverBy time.Time) (time.Time, time.Time) {
	local := deliverBy.In(jst.Zone)
	first := time.Date(local.Year(), local.Month()+1, 15, 0, 0, 0, 0, jst.Zone)
	return first, first.AddDate(0, 1, 0)
}

// buildWorkOrderTerms は、委託の内容から9項目を作る。consignedAt は引き受けた時(打診中は今)。
func buildWorkOrderTerms(coachName, work string, amount int32, deliverBy, consignedAt time.Time) workOrderTerms {
	due, carried := payoutDates(deliverBy)
	if strings.TrimSpace(coachName) == "" {
		coachName = "（コーチとして登録した名前）"
	}
	return workOrderTerms{
		AmountYen:   amount,
		DeliverBy:   deliverBy,
		PaymentDue:  due,
		CarriedDue:  carried,
		ConsignedAt: consignedAt,
		Items: []workOrderItem{
			{"委託する人・受ける人", fmt.Sprintf("委託する人: %s（TrackTalk の運営者）／受ける人: %s", operatorName, coachName)},
			{"委託した日", jstDateTime(consignedAt) + "（引き受けた日時）"},
			{"業務の内容", work},
			{"納める期日", jstDateTime(deliverBy) + "まで"},
			{"納める場所", "TrackTalk のコーチ専用画面から送信します"},
			{"検査", "運営者は、納められた内容の検査をしません。送信した時に受け取ったものとします"},
			{"報酬の額", fmt.Sprintf("%d円（税込）", amount)},
			{"支払期日", fmt.Sprintf("%s。ただし、前の月の末日までに確定した報酬の合計が3,000円に満たないときは翌月に繰り越し、%sに支払います（繰り越しは1回まで）。Stripe での本人確認と口座の登録が済んでいない間は、済んだ後の支払日に支払います", jstDate(due), jstDate(carried))},
			{"支払方法", "Stripe を通じて、登録した銀行口座に振り込みます。振込の手数料は運営者が負担します"},
		},
	}
}

// reviewWorkOrderTerms は、フォーム添削の依頼の通知。報酬は依頼のときに確定した割引前の価格と報酬率から出す。
func (s *Server) reviewWorkOrderTerms(ctx context.Context, q *db.Queries, rr db.ReviewRequest, coachName string, consignedAt time.Time) (workOrderTerms, error) {
	price, share := rr.BasePriceYen, int32(reviewSharePercent)
	t, err := q.GetReviewTicketTerms(ctx, &rr.ID)
	if err == nil {
		price, share = t.NominalYen, int32(t.SharePercent)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return workOrderTerms{}, err
	}
	what := "見てほしい点: " + strings.Join(rr.Focus, "、")
	if strings.TrimSpace(rr.Question) != "" {
		what = "相談の内容に答える"
	}
	work := fmt.Sprintf("フォーム添削。利用者が送った動画1本（種目 %s、%s）を見て、文章で添削を返す。返却後に利用者から確認の質問が来たときは、補足を1回返す", rr.Event, what)
	return buildWorkOrderTerms(coachName, work, price*share/100, rr.DueAt, consignedAt), nil
}

// consultationWorkOrderTerms は、フォーム添削の追加相談の通知。回答の期限は引き受けてから72時間。
func consultationWorkOrderTerms(c db.CoachingConsultation, coachName string, consignedAt time.Time) workOrderTerms {
	deliverBy := consignedAt.Add(72 * time.Hour)
	if c.AnswerBy != nil {
		deliverBy = *c.AnswerBy
	}
	work := "フォーム添削の追加相談。利用者の質問1件に、文章（実践動画は任意）で1回答える。回答への確認の質問が来たときは、補足を1回返す"
	return buildWorkOrderTerms(coachName, work, int32(int64(c.PriceYen)*int64(c.SharePercent)/100), deliverBy, consignedAt)
}

// coachDisplayName は、通知に書くコーチの名前。登録した名前がなければニックネーム。
func coachDisplayName(ctx context.Context, q *db.Queries, coachID uuid.UUID) (string, error) {
	cp, err := q.GetCoachProfile(ctx, coachID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cp.Name) != "" {
		return cp.Name, nil
	}
	p, err := q.GetProfileByID(ctx, coachID)
	if err != nil {
		return "", err
	}
	if p.Nickname != nil {
		return *p.Nickname, nil
	}
	return "", nil
}

// saveWorkOrder は、引き受けた案件の通知を書き留める。すでにあれば最初のものを残す。
func saveWorkOrder(ctx context.Context, q *db.Queries, kind string, review, consultation *uuid.UUID, coachID uuid.UUID, terms workOrderTerms) error {
	raw, err := json.Marshal(terms)
	if err != nil {
		return err
	}
	_, err = q.CreateCoachWorkOrder(ctx, db.CreateCoachWorkOrderParams{
		ID: uuid.New(), Kind: kind, ReviewRequestID: review, ConsultationID: consultation,
		CoachID: &coachID, TermsVersion: workOrderTermsVersion, Terms: raw,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func workOrderJSON(w db.CoachWorkOrder) map[string]any {
	return map[string]any{
		"id":            w.ID,
		"terms_version": w.TermsVersion,
		"terms":         json.RawMessage(w.Terms),
		"created_at":    w.CreatedAt,
	}
}

// attachConsultationWorkOrder は、コーチに見せる追加相談に取引条件を付ける。
// 引き受ける前は今の条件(terms)、引き受けた後は書き留めた通知(work_order)。
func (s *Server) attachConsultationWorkOrder(ctx context.Context, out map[string]any, c db.CoachingConsultation) error {
	if c.Status == "offered" {
		name, err := coachDisplayName(ctx, s.q, *c.CoachID)
		if err != nil {
			return err
		}
		out["terms"] = consultationWorkOrderTerms(c, name, time.Now())
		return nil
	}
	wo, err := s.q.GetConsultationWorkOrder(ctx, db.GetConsultationWorkOrderParams{ConsultationID: &c.ID, CoachID: c.CoachID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	out["work_order"] = workOrderJSON(wo)
	return nil
}
