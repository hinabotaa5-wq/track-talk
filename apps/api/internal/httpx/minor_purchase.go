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

// 18歳未満が自分で払う動画添削・追加相談の、購入のたびの宣言。
// 保護者同意と月の上限で止める代わりに、支払い画面を作る前に「保護者に許可をもらいました」へのチェックを必須にし、
// チェックした記録を取引ごとに残す（minor_purchase_declarations）。18歳以上には求めない。
// 年齢の判定は、これまでどおり自己申告の生まれた年月で、依頼を作る時・支払い画面を作る前のロックの中・追加相談の支払いが届いた時に行う。
// 保護者が契約する指導（パーソナル指導・サポートプラン）には使わない。

// minorPurchaseDeclarationVersion は、宣言の文案の版。画面（apps/web/lib/minor-purchase.ts）の文案を変えたら両方を上げる。
const minorPurchaseDeclarationVersion = "minor-purchase-declaration-2026-11-05"

// minorDeclarationReq は、購入の操作で一緒に送る宣言。
type minorDeclarationReq struct {
	GuardianPermission bool `json:"guardian_permission"`
}

// errMinorDeclarationRequired は、18歳未満が宣言にチェックせずに購入しようとしたとき。
var errMinorDeclarationRequired = &ticketRuleError{"guardian_permission_required", "購入の前に、保護者の方に許可をもらったことを確認して、チェックを入れてください。"}

// profileIsMinor は、自己申告の生まれた年月で18歳未満かを返す。生まれた年月がないときは false（依頼の前に別に止める）。
func profileIsMinor(p db.Profile) bool {
	return p.BirthMonth != nil && isMinor(*p.BirthMonth, time.Now())
}

// requireMinorDeclaration は、18歳未満なのに宣言にチェックがないときにエラーを返す。
func requireMinorDeclaration(minor, declared bool) error {
	if minor && !declared {
		return errMinorDeclarationRequired
	}
	return nil
}

// decodeMinorDeclaration は、本文のない操作（追加相談の送信・支払い）で、任意の本文から宣言を読む。本文が空なら宣言なし。
func decodeMinorDeclaration(r *http.Request) (bool, error) {
	var req minorDeclarationReq
	if err := DecodeJSON(r, &req); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	return req.GuardianPermission, nil
}

// consultationEligibility は、追加相談の操作ができる人かを確かめる。18歳未満の動画添削のあとの相談を
// その場で払う（cash）ときは保護者の同意を見ず、購入のたびの宣言で確かめる。チケットで払う相談と、
// 保護者が契約する指導の相談は、これまでどおり保護者の同意も確かめる。
func (s *Server) consultationEligibility(ctx context.Context, q *db.Queries, owner db.Profile, reviewRoom bool, funding string) error {
	if reviewRoom && funding != "ticket" {
		return coachingEligibility(owner)
	}
	return s.ticketEligibility(ctx, q, owner)
}

// recordMinorDeclaration は、宣言を1行残す。支払い画面を作るのと同じトランザクションの q を渡す
// （残せなかったら支払い画面も作らない）。
func recordMinorDeclaration(ctx context.Context, q *db.Queries, minorID uuid.UUID, refType string, refID uuid.UUID) error {
	return q.InsertMinorPurchaseDeclaration(ctx, db.InsertMinorPurchaseDeclarationParams{
		ID: uuid.New(), MinorID: minorID, RefType: refType, RefID: refID, TextVersion: minorPurchaseDeclarationVersion,
	})
}
