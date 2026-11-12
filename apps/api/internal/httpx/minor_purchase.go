package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

// (1) 動画添削・チケットの購入前チェック。年齢による分岐はしない。
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

// requirePurchaseNotice は、決済前のチェックがないときにエラーを返す。
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

// purchaseNoticeVersion は年齢によらない購入前チェックの版。旧宣言とは保持方法を区別する。
const purchaseNoticeVersion = "purchase-notice-2026-11-13"

func recordPurchaseNotice(ctx context.Context, q *db.Queries, account db.Profile, purchase minorPurchase, decl minorDeclarationReq) error {
	now := time.Now()
	label := "未成年の方は保護者に許可を取ってください"
	amount := purchase.amount
	return q.InsertMinorPurchaseDeclaration(ctx, db.InsertMinorPurchaseDeclarationParams{
		ID: uuid.New(), MinorID: account.ID, RefType: purchase.refType, RefID: purchase.refID, TextVersion: purchaseNoticeVersion,
		DeclarationText: &label, AmountYen: &amount, CoachID: purchase.coachID,
		ClientShownAt: clientTime(decl.ShownAt, now), ClientCheckedAt: clientTime(decl.CheckedAt, now),
	})
}
