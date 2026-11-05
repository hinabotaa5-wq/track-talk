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

// 宣言の文面（チェックの横の宣言文と、その上の注意書き）。画面と同じ文字列にする（テストで突き合わせる）。
// 版だけでなく文面そのものも記録に写す。
const (
	minorPurchaseDeclarationLabel = "保護者に許可をもらいました"
	minorPurchaseDeclarationNote  = "未成年の方は、購入する前に保護者の許可を得てください。"
)

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
// （残せなかったら支払い画面も作らない）。記録は年齢確認・同意確認の措置を講じたことの証跡で、
// 宣言の文面・取引の金額とコーチ・その時の生まれた年月・画面での表示とチェックの時刻を写す。
// 詐術を理由に返金を断るためのものではない（返金の方針は変えない）。
func recordMinorDeclaration(ctx context.Context, q *db.Queries, minor db.Profile, purchase minorPurchase, decl minorDeclarationReq) error {
	now := time.Now()
	label, note := minorPurchaseDeclarationLabel, minorPurchaseDeclarationNote
	amount := purchase.amount
	return q.InsertMinorPurchaseDeclaration(ctx, db.InsertMinorPurchaseDeclarationParams{
		ID: uuid.New(), MinorID: minor.ID, RefType: purchase.refType, RefID: purchase.refID, TextVersion: minorPurchaseDeclarationVersion,
		DeclarationText: &label, NoticeText: &note, AmountYen: &amount, CoachID: purchase.coachID,
		DeclaredBirthMonth: minor.BirthMonth, ClientShownAt: clientTime(decl.ShownAt, now), ClientCheckedAt: clientTime(decl.CheckedAt, now),
	})
}
