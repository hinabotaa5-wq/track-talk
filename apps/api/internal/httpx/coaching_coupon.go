package httpx

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// 紹介特典の ¥500 クーポン（本体 #1031）。紹介の周を達成すると1枚付与する（DB の coaching_grant_referral）。
// クーポンは運営が負担し、コーチ報酬は割引前の価格 × 70%（reviewSharePercent）で計算する。
// 1回の申込に使えるのは1枚まで、チケットとの併用はできない。依頼が不成立で終わったら、
// チケットと同じ時点・条件（settleReviewTickets の戻し）で未使用に戻す。

// referralCouponYen は、紹介特典のクーポンの額面。DB の coaching_grant_referral（000192）と揃える。
const referralCouponYen int32 = 500

// referralRewardTerms は、紹介特典のクーポンの条件。付与の通知と画面に出す。
const referralRewardTerms = "¥500クーポン：付与日から6か月有効／付与された本人だけが使えます／現金への払戻しはできません／1回の申込に1枚まで"

// legacyRewardTicketTerms は、発行済みの特典チケット（無料チケット）の条件。発行時の条件のまま有効期限まで使える。
const legacyRewardTicketTerms = "特典チケット：付与日から6か月有効／付与された本人だけが使えます／払戻しはできません"

// couponCashPrice は、価格 price にクーポン discount を適用したときの選手の支払額（0円未満にはしない）。
func couponCashPrice(price, discount int32) int32 {
	return max(price-discount, 0)
}

// reviewNeedsPayment は、申込に支払い画面が要るか。クーポンで支払額が0円になったときは支払いを飛ばす。
func reviewNeedsPayment(stripeConfigured bool, cashPrice int32) bool {
	return stripeConfigured && cashPrice > 0
}

// couponDiscount は、価格 price の申込に額面 amount のクーポンを使ったときの割引額（価格を超えない）。
func couponDiscount(price, amount int32) int32 {
	return min(price, amount)
}

// couponJSON は、本人に見せるクーポンの保有状況（枚数・いちばん早い失効日・額面）。
func couponJSON(b db.CouponBalanceRow) map[string]any {
	var next any
	if b.Available > 0 {
		next = b.NextExpiresAt
	}
	return map[string]any{"available": b.Available, "next_expires_at": next, "amount_yen": referralCouponYen, "terms": referralRewardTerms}
}

// reserveReviewCoupon は、依頼にクーポン1枚を確保する。呼び出し側は利用者のロックを持ち、依頼の作成と同じトランザクションで呼ぶ。
// 指定（couponID）があればその1枚、なければ期限の早いものを使う。他人の・使用済・確保中・失効したクーポンは使えない。
func reserveReviewCoupon(ctx context.Context, q *db.Queries, owner uuid.UUID, couponID *uuid.UUID, rr db.ReviewRequest) error {
	c, err := q.LockUsableCoupon(ctx, db.LockUsableCouponParams{OwnerID: owner, CouponID: couponID})
	if errors.Is(err, pgx.ErrNoRows) {
		return &ticketRuleError{"coupon_unavailable", "使えるクーポンがありません。お支払い方法を選び直してください"}
	}
	if err != nil {
		return err
	}
	if couponDiscount(rr.BasePriceYen, c.AmountYen) != rr.CouponDiscountYen {
		return &ticketRuleError{"coupon_unavailable", "クーポンの内容が変わりました。画面を更新してください"}
	}
	return q.ReserveCoupon(ctx, db.ReserveCouponParams{ID: c.ID, RequestID: &rr.ID})
}

// consumeReviewCoupon は、返却した依頼のクーポンを使用済にする。支払いが0円になった依頼の報酬もここで記録する
// （支払いのある依頼の報酬は、capture 後に InsertReviewEarnings が割引前の価格で記録する）。
func consumeReviewCoupon(ctx context.Context, q *db.Queries, requestID uuid.UUID) error {
	c, err := q.GetReservedCouponForReview(ctx, &requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := q.UseCoupon(ctx, c.ID); err != nil {
		return err
	}
	return q.RecordCouponReviewEarnings(ctx, db.RecordCouponReviewEarningsParams{SharePercent: reviewSharePercent, RequestID: requestID})
}

// releaseReviewCoupon は、不成立で終わった依頼のクーポンを戻す。チケットの戻し（settleReviewTickets）と同じ条件にする：
//   - 紹介の対象購入が返金されて特典が取り消されていたら、戻さずに取り消す。
//   - 戻す時点で期限を過ぎていたら、戻さずに失効させる。運営者・コーチ都合の終了なら新しい1枚を出し直す（#797）。
//   - それ以外は未使用に戻す。
func releaseReviewCoupon(ctx context.Context, q *db.Queries, id uuid.UUID) error {
	c, err := q.LockCoupon(ctx, id)
	if err != nil {
		return err
	}
	if c.Status != "reserved" {
		return nil
	}
	refunded, err := q.CouponRewardHasRefund(ctx, &c.ID)
	if err != nil {
		return err
	}
	if refunded {
		if err := q.EndCoupon(ctx, db.EndCouponParams{Status: "canceled", ID: c.ID}); err != nil {
			return err
		}
		return q.CancelCouponReward(ctx, &c.ID)
	}
	if c.ExpiresAt.After(time.Now()) {
		return q.ReleaseCoupon(ctx, c.ID)
	}
	coachCaused, err := coachCausedEnd(ctx, q, c.RequestID)
	if err != nil {
		return err
	}
	if err := q.EndCoupon(ctx, db.EndCouponParams{Status: "expired", ID: c.ID}); err != nil {
		return err
	}
	if !coachCaused || c.OwnerID == nil {
		return nil
	}
	return reissueExpiredCoupon(ctx, q, c)
}

// releaseReviewCouponFor は、依頼に確保しているクーポンがあれば戻す（支払い画面を作れずに依頼を消すとき）。
func releaseReviewCouponFor(ctx context.Context, q *db.Queries, requestID uuid.UUID) error {
	c, err := q.GetReservedCouponForReview(ctx, &requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return releaseReviewCoupon(ctx, q, c.ID)
}

// reissueExpiredCoupon は、運営者・コーチ都合で終わった依頼で失効に回ったクーポンを、出し直した日から6か月有効の
// 新しい1枚として出し直す（チケットの #797 と同じ。旧クーポンの期限延長ではない）。
func reissueExpiredCoupon(ctx context.Context, q *db.Queries, old db.CoachingCoupon) error {
	if err := q.CreateReissueCoupon(ctx, db.CreateReissueCouponParams{ID: uuid.New(), OwnerID: old.OwnerID, AmountYen: old.AmountYen}); err != nil {
		return err
	}
	wanted, err := notificationWanted(ctx, q, *old.OwnerID, notifyCoaching, true)
	if err != nil || !wanted {
		return err
	}
	title := "紹介特典：期限切れのクーポンを出し直しました"
	body := "運営者・コーチの都合で終わったスタンダードで使った分を出し直しました。新しい" + referralRewardTerms
	url, role := "/me/coaching/tickets", "player"
	return q.InsertMessageNotification(ctx, db.InsertMessageNotificationParams{
		ID: uuid.New(), RecipientID: *old.OwnerID, Title: &title, Body: &body, Url: &url, CoachingRole: &role,
	})
}

// releaseEndedCoupons は tick から呼ぶ。確保中のまま依頼が不成立で終わった（expired・依頼が消えた）クーポンを戻す。
func (s *Server) releaseEndedCoupons(ctx context.Context) (int, error) {
	rows, err := s.q.ListCouponsToRelease(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, row := range rows {
		released := false
		err := s.tx(ctx, func(q *db.Queries) error {
			// 回答処理と同じ順（依頼→クーポン）でロックし、まだ終わっていない依頼のクーポンは戻さない。
			if row.RequestID != nil {
				rr, e := q.LockTicketReview(ctx, *row.RequestID)
				if e != nil && !errors.Is(e, pgx.ErrNoRows) {
					return e
				}
				if e == nil && rr.Status != "expired" {
					return nil
				}
			}
			c, err := q.LockCoupon(ctx, row.ID)
			if err != nil || c.Status != "reserved" || (c.RequestID != nil && (row.RequestID == nil || *c.RequestID != *row.RequestID)) {
				return err
			}
			if err := releaseReviewCoupon(ctx, q, row.ID); err != nil {
				return err
			}
			released = true
			return nil
		})
		if err != nil {
			return n, err
		}
		if released {
			n++
		}
	}
	return n, nil
}

// expireCoupons は tick から呼ぶ。有効期限を過ぎた未使用のクーポンを失効させる。
func (s *Server) expireCoupons(ctx context.Context) (int, error) {
	n, err := s.q.ExpireUnusedCoupons(ctx)
	return int(n), err
}
