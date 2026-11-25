package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// (1) 動画添削・追加質問の購入前チェック。未成年（の可能性がある人）にだけ求める（本体 #1045）。
// 保護者が契約するサポートプランの同意には使わない。

// minorDeclarationReq は、購入の操作で一緒に送る宣言。ShownAt・CheckedAt は、画面に宣言を出した時刻と
// チェックを入れた時刻（利用者の端末の時計）。証跡として残すだけで、有無や値で購入を止めない。
type minorDeclarationReq struct {
	GuardianPermission bool       `json:"guardian_permission"`
	ShownAt            *time.Time `json:"guardian_permission_shown_at,omitempty"`
	CheckedAt          *time.Time `json:"guardian_permission_checked_at,omitempty"`
}

// minorPurchase は、宣言の記録に写す取引の中身。
type minorPurchase struct {
	refType string
	refID   uuid.UUID
	amount  int32
	coachID *uuid.UUID
}

// errMinorDeclarationRequired は、購入前チェックがないときの互換エラーコード。
var errMinorDeclarationRequired = &ticketRuleError{"guardian_permission_required", "購入前の注意事項を確認して、チェックを入れてください。"}

// birthYearKnown は、年齢の判定に使える年の情報（birth_year か、旧方式の birth_month）があるか。
// ない人はプロフィール画面で生まれた年を登録するまで、ほかの画面へ進めない（Web のゲート）。
func birthYearKnown(p db.Profile) bool {
	return p.BirthYear != nil || p.BirthMonth != nil
}

// purchaseNoticeRequired は、(1) の購入前チェックを求める相手か（本体 #1045）。クライアントの申告ではなく profile から決める。
//   - birth_month がある人は、月精度の isMinor をそのまま使う（サポートプランと同じ判定）。
//   - birth_year だけの人は、日本時間の今年 − 生まれた年 が 18 以下の間を「未成年の可能性あり」とする
//     （今年18歳になる人は誕生日の前は17歳なので含める。19以上なら確実に成人）。
//   - 年の情報がない人（画面のゲートを通らず API を直接呼んだ場合）は、安全側でチェックを求める。
func purchaseNoticeRequired(p db.Profile, now time.Time) bool {
	if p.BirthMonth != nil {
		return isMinor(*p.BirthMonth, now)
	}
	if p.BirthYear != nil {
		return now.In(jst.Zone).Year()-int(*p.BirthYear) <= 18
	}
	return true
}

// requirePurchaseNotice は、決済前のチェックが必要なのにないときにエラーを返す。
// required には「支払いがあり、かつ purchaseNoticeRequired」の結果を渡す。
func requirePurchaseNotice(required, declared bool) error {
	if required && !declared {
		return errMinorDeclarationRequired
	}
	return nil
}

// decodeMinorDeclaration は、本文のない操作（追加相談の送信・支払い）で、任意の本文から宣言を読む。本文が空なら宣言なし。
func decodeMinorDeclaration(r *http.Request) (minorDeclarationReq, error) {
	var req minorDeclarationReq
	if err := DecodeJSON(r, &req); err != nil {
		if errors.Is(err, io.EOF) {
			return minorDeclarationReq{}, nil
		}
		return minorDeclarationReq{}, err
	}
	return req, nil
}

// clientTime は、端末から届いた時刻を記録に使えるものだけ通す。届いた時より1日以上前や、
// 5分以上先のもの（端末の時計の大きな狂いや作った値）は記録しない。
func clientTime(t *time.Time, now time.Time) *time.Time {
	if t == nil || t.Before(now.Add(-24*time.Hour)) || t.After(now.Add(5*time.Minute)) {
		return nil
	}
	v := t.UTC()
	return &v
}

// consultationEligibility は (1) の年齢ゲートを置かず、(2) の既存同意確認だけを維持する。
func (s *Server) consultationEligibility(ctx context.Context, q *db.Queries, owner db.Profile, reviewRoom bool, funding string) error {
	if reviewRoom {
		return nil
	}
	return s.ticketEligibility(ctx, q, owner)
}

// purchaseNoticeVersion は購入前チェックの版。2026-11-13 版は全員に表示、2026-11-26 版から未成年（の可能性がある人）だけに表示する（本体 #1045）。
// 旧宣言（purchase-notice より前の版）とは保持方法を区別する。
const purchaseNoticeVersion = "purchase-notice-2026-11-26"

// recordPurchaseNotice は、チェックを求めた購入だけ記録を残す。成人と確定できる人の購入では記録を作らない（本体 #1045）。
func recordPurchaseNotice(ctx context.Context, q *db.Queries, account db.Profile, purchase minorPurchase, decl minorDeclarationReq) error {
	now := time.Now()
	if !purchaseNoticeRequired(account, now) {
		return nil
	}
	label := "未成年の方は保護者に許可を取ってください"
	amount := purchase.amount
	return q.InsertMinorPurchaseDeclaration(ctx, db.InsertMinorPurchaseDeclarationParams{
		ID: uuid.New(), MinorID: account.ID, RefType: purchase.refType, RefID: purchase.refID, TextVersion: purchaseNoticeVersion,
		DeclarationText: &label, AmountYen: &amount, CoachID: purchase.coachID,
		ClientShownAt: clientTime(decl.ShownAt, now), ClientCheckedAt: clientTime(decl.CheckedAt, now),
	})
}
