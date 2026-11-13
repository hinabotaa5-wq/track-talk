package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5"
)

// 取引条件の通知(フリーランス保護法3条)。運営者がフォーム添削とその追加質問をコーチに委託するたびに、
// その案件の9項目を、引き受ける前の打診の画面に出し、引き受けたら書き留め(coach_work_orders)、
// 同時にコーチのお知らせへ宛先付きで届けて、届けた記録を残す(coach_work_order_deliveries)。
// 公正取引委員会の Q&A(Q41)で認められている、受信者を特定した送信(アプリのメッセージ機能・URL の記載)に寄せるため。
// 弁護士の回答で「利用規約への同意だけでは足りず、案件ごとの明示が要る」とされたため。
// サポートプランはコーチが販売者なので、ここには入れない(要否は弁護士の回答 Q9 を待つ)。

// 通知の文面の版。文面を変えたら上げる。書き留めた通知は、そのときの版のまま残る。
// 4: お知らせ(ベル)の文面を「依頼が届いた」の1行だけにし、9項目の全文は依頼の画面にだけ出す形にした(本体 #609)。
// 5: 極短文面(本体 #625)。委託者・受託者は役割表示のみで個人名を出さない。3条の9要素は8行にまとめる(支払期日と支払方法は「支払」1行)。
const workOrderTermsVersion = 5

type workOrderItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type workOrderTerms struct {
	Items       []workOrderItem `json:"items"`
	AmountYen   int32           `json:"amount_yen"`
	DeliverBy   time.Time       `json:"deliver_by"`
	PaymentDue  time.Time       `json:"payment_due"`
	ConsignedAt time.Time       `json:"consigned_at"`
}

func jstDateTime(t time.Time) string {
	return t.In(jst.Zone).Format("2006年1月2日 15:04")
}

// payoutDate は、deliverBy までに確定した報酬を支払う日(日本時間の翌月15日)。
// 3,000円未満で繰り越したときの支払日は、累積が3,000円以上になる月で決まるので、ここでは出さない。
func payoutDate(deliverBy time.Time) time.Time {
	local := deliverBy.In(jst.Zone)
	return time.Date(local.Year(), local.Month()+1, 15, 0, 0, 0, 0, jst.Zone)
}

// workOrderPaymentShort は、3条の明示の「支払期日」と「支払方法」を1行にまとめた極短文面(本体 #625)。
const workOrderPaymentShort = "月末締・翌月15日、累積¥3,000未満は繰越。Stripe→登録口座、手数料運営負担。未登録分は登録後の支払日"

// buildWorkOrderTerms は、委託の内容から極短の8行を作る(本体 #625)。個人名は入れない。consignedAt は引き受けた時(打診中は今)。
func buildWorkOrderTerms(work string, amount int32, deliverBy, consignedAt time.Time) workOrderTerms {
	due := payoutDate(deliverBy)
	return workOrderTerms{
		AmountYen:   amount,
		DeliverBy:   deliverBy,
		PaymentDue:  due,
		ConsignedAt: consignedAt,
		Items: []workOrderItem{
			{"委託", "TrackTalk運営者／受託: 担当コーチ"},
			{"日", jstDateTime(consignedAt)},
			{"内容", work},
			{"期限", jstDateTime(deliverBy)},
			{"場所", "この画面"},
			{"検査", "なし（送信で受領）"},
			{"報酬", fmt.Sprintf("¥%d", amount)},
			{"支払", workOrderPaymentShort},
		},
	}
}

// reviewWorkOrderTerms は、フォーム添削の依頼の通知。報酬は依頼のときに確定した割引前の価格と報酬率から出す。
func (s *Server) reviewWorkOrderTerms(ctx context.Context, q *db.Queries, rr db.ReviewRequest, consignedAt time.Time) (workOrderTerms, error) {
	price, share := rr.BasePriceYen, int32(reviewSharePercent)
	t, err := q.GetReviewTicketTerms(ctx, &rr.ID)
	if err == nil {
		price, share = t.NominalYen, int32(t.SharePercent)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return workOrderTerms{}, err
	}
	work := fmt.Sprintf("動画1本の添削（%s）。相談に答えて文章で返却。確認質問に補足1回", rr.Event)
	return buildWorkOrderTerms(work, price*share/100, rr.DueAt, consignedAt), nil
}

// consultationWorkOrderTerms は、フォーム添削の追加質問の通知。回答の期限は引き受けてから72時間。
// 内容行は現行差分のまま(補足1回の文は入れない。本体 #625)。
func consultationWorkOrderTerms(c db.CoachingConsultation, consignedAt time.Time) workOrderTerms {
	deliverBy := consignedAt.Add(72 * time.Hour)
	if c.AnswerBy != nil {
		deliverBy = *c.AnswerBy
	}
	work := "フォーム添削の追加質問。利用者の質問1件に、文章（実践動画は任意）で1回答える"
	return buildWorkOrderTerms(work, int32(int64(c.PriceYen)*int64(c.SharePercent)/100), deliverBy, consignedAt)
}

// saveWorkOrder は、引き受けた案件の通知を書き留め、同じトランザクションでコーチへ宛先付きで届ける。
// すでにあれば最初のものを残し、届け直さない。roomID は条件の全文を見られる依頼の画面の id。
// プッシュは送らない(依頼・追加相談が届いた知らせの側に一本化する。本体 #609)。
func saveWorkOrder(ctx context.Context, q *db.Queries, kind string, review, consultation *uuid.UUID, coachID, roomID uuid.UUID, terms workOrderTerms) error {
	raw, err := json.Marshal(terms)
	if err != nil {
		return err
	}
	wo, err := q.CreateCoachWorkOrder(ctx, db.CreateCoachWorkOrderParams{
		ID: uuid.New(), Kind: kind, ReviewRequestID: review, ConsultationID: consultation,
		CoachID: &coachID, TermsVersion: workOrderTermsVersion, Terms: raw,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return deliverWorkOrder(ctx, q, wo, coachID, roomID)
}

// workOrderNotice は、取引条件を届けるお知らせの見出し・本文・開く先。
// お知らせは「何が届いたか」の1行だけにして本文は空にし、9項目の全文は開く先の依頼の画面
// (コーチ本人だけが見られる)にだけ出す(本体 #609)。
func workOrderNotice(kind string, roomID uuid.UUID) (title, body, url string) {
	title = "動画添削の依頼が届きました"
	if kind == "consultation" {
		title = "追加質問が届きました"
	}
	url = "/coach/requests/" + roomID.String() + "#work-order"
	return title, "", url
}

// deliverWorkOrder は、書き留めた通知をコーチのお知らせ(ベル)に入れ、届けた記録を残す。
// フリーランス保護法3条の電磁的方法による明示(受信者を特定した送信)にあたるので、
// 本人の通知の設定に関わらず届け、記録とあわせて引き受けと同じトランザクションで行う。
func deliverWorkOrder(ctx context.Context, q *db.Queries, wo db.CoachWorkOrder, coachID, roomID uuid.UUID) error {
	title, body, url := workOrderNotice(wo.Kind, roomID)
	if err := q.InsertWorkOrderNotification(ctx, db.InsertWorkOrderNotificationParams{
		ID: uuid.New(), RecipientID: coachID, Title: &title, Body: &body, Url: &url,
	}); err != nil {
		return err
	}
	_, err := q.CreateWorkOrderDelivery(ctx, db.CreateWorkOrderDeliveryParams{
		ID: uuid.New(), WorkOrderID: wo.ID, RecipientID: &coachID, TermsVersion: wo.TermsVersion,
		Title: title, Body: body, Url: url,
	})
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

// workOrderWithDelivery は、書き留めた通知に、お知らせで届けた日時を添える(届けた記録がなければ添えない)。
func workOrderWithDelivery(ctx context.Context, q *db.Queries, w db.CoachWorkOrder) (map[string]any, error) {
	out := workOrderJSON(w)
	d, err := q.GetWorkOrderDelivery(ctx, w.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out["delivered_at"] = d.DeliveredAt
	return out, nil
}

// attachConsultationWorkOrder は、コーチに見せる追加質問に取引条件を付ける。
// 引き受ける前は今の条件(terms)、引き受けた後は書き留めた通知(work_order)。
func (s *Server) attachConsultationWorkOrder(ctx context.Context, out map[string]any, c db.CoachingConsultation) error {
	if c.Status == "offered" {
		out["terms"] = consultationWorkOrderTerms(c, time.Now())
		return nil
	}
	wo, err := s.q.GetConsultationWorkOrder(ctx, db.GetConsultationWorkOrderParams{ConsultationID: &c.ID, CoachID: c.CoachID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	order, err := workOrderWithDelivery(ctx, s.q, wo)
	if err != nil {
		return err
	}
	out["work_order"] = order
	return nil
}
