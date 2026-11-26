package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// コーチの価格申請制（本体 #1052）のテスト。T1〜T10 は Issue の番号。

type coachPricesView struct {
	Plans []struct {
		Plan      string `json:"plan"`
		PriceYen  *int   `json:"price_yen"`
		IsDefault bool   `json:"is_default"`
		Pending   *struct {
			ID       string `json:"id"`
			PriceYen int    `json:"price_yen"`
		} `json:"pending"`
		Approved *struct {
			ID       string `json:"id"`
			PriceYen int    `json:"price_yen"`
		} `json:"approved"`
	} `json:"plans"`
	MinPriceYen int `json:"min_price_yen"`
}

type adminCoachPricesView struct {
	Applications []struct {
		ID                 string  `json:"id"`
		CoachID            string  `json:"coach_id"`
		Plan               string  `json:"plan"`
		PriceYen           int     `json:"price_yen"`
		Source             string  `json:"source"`
		Status             string  `json:"status"`
		ApprovedAt         *string `json:"approved_at"`
		ApprovedByNickname *string `json:"approved_by_nickname"`
	} `json:"applications"`
}

func myCoachPrices(t *testing.T, h http.Handler, tc string) coachPricesView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/prices", tc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("my prices %d %s", rec.Code, rec.Body.String())
	}
	var out coachPricesView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func adminCoachPrices(t *testing.T, h http.Handler, admin string) adminCoachPricesView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/coach-prices", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin prices %d %s", rec.Code, rec.Body.String())
	}
	var out adminCoachPricesView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// pendingPriceID は、運営の一覧からコーチ×プランの承認待ちの申請 ID を探す。
func pendingPriceID(t *testing.T, h http.Handler, admin string, coach uuid.UUID, plan string) string {
	t.Helper()
	for _, a := range adminCoachPrices(t, h, admin).Applications {
		if a.CoachID == coach.String() && a.Plan == plan && a.Status == "pending" {
			return a.ID
		}
	}
	t.Fatalf("no pending %s price for %s", plan, coach)
	return ""
}

func approveCoachPrice(t *testing.T, h http.Handler, admin, id string) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coach-prices/"+id+"/approve", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve price %d %s", rec.Code, rec.Body.String())
	}
}

func planState(t *testing.T, v coachPricesView, plan string) (price *int, pending, approved *int) {
	t.Helper()
	for _, p := range v.Plans {
		if p.Plan == plan {
			if p.Pending != nil {
				pending = &p.Pending.PriceYen
			}
			if p.Approved != nil {
				approved = &p.Approved.PriceYen
			}
			return p.PriceYen, pending, approved
		}
	}
	t.Fatalf("plan %s missing in %+v", plan, v)
	return nil, nil, nil
}

func requestBasePrice(t *testing.T, id string) int {
	t.Helper()
	var base int
	queryRowArgs(t, `SELECT base_price_yen FROM review_requests WHERE id=$1`, []any{id}, &base)
	return base
}

func returnReview(t *testing.T, h http.Handler, tc, id string) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != http.StatusOK {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
}

func notificationCount(t *testing.T, recipient uuid.UUID, title string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM notifications WHERE recipient_id=$1 AND title=$2`, []any{recipient, title}, &n)
	return n
}

// T1・T2・T3：申請・運営の一覧・上書き・取り下げ・下限。承認後はコーチから変えられない。
func TestCoachPriceApplyOverwriteWithdraw(t *testing.T) {
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)

	// T8 の前提：未設定のコーチは共通の1,000円（スタンダード）・未設定（プレミアム）。
	v := myCoachPrices(t, h, tc)
	if price, pending, approved := planState(t, v, "standard"); price == nil || *price != 1000 || pending != nil || approved != nil {
		t.Fatalf("initial standard %+v", v)
	}
	if price, _, _ := planState(t, v, "premium"); price != nil {
		t.Fatalf("initial premium %+v", v)
	}
	if v.MinPriceYen != 500 {
		t.Fatalf("min %d", v.MinPriceYen)
	}

	// T3：499円以下は弾き、500円は通る。上限はない。
	for _, bad := range []any{499, 0, -1, nil} {
		rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/standard", tc, map[string]any{"price_yen": bad})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "price_yen") {
			t.Fatalf("bad %v: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/standard", tc, map[string]any{"price_yen": 500}); rec.Code != http.StatusOK {
		t.Fatalf("500 %d %s", rec.Code, rec.Body.String())
	}
	if _, pending, _ := planState(t, myCoachPrices(t, h, tc), "standard"); pending == nil || *pending != 500 {
		t.Fatal("pending 500 missing")
	}

	// T1：運営の一覧に出る。
	id := pendingPriceID(t, h, admin, coach, "standard")

	// T2：申請中は上書きでき（同じ1件の書き換え）、大きな額も通る。
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/standard", tc, map[string]any{"price_yen": 50_000_000}); rec.Code != http.StatusOK {
		t.Fatalf("overwrite %d %s", rec.Code, rec.Body.String())
	}
	if again := pendingPriceID(t, h, admin, coach, "standard"); again != id {
		t.Fatalf("overwrite created a new application %s != %s", again, id)
	}
	if _, pending, _ := planState(t, myCoachPrices(t, h, tc), "standard"); pending == nil || *pending != 50_000_000 {
		t.Fatal("overwrite not reflected")
	}
	var pendings int
	queryRowArgs(t, `SELECT count(*) FROM coach_price_applications WHERE coach_id=$1 AND status='pending'`, []any{coach}, &pendings)
	if pendings != 1 {
		t.Fatalf("pending rows %d", pendings)
	}

	// T2：取り下げられる。取り下げたあとは承認できない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/prices/standard/withdraw", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	if _, pending, _ := planState(t, myCoachPrices(t, h, tc), "standard"); pending != nil {
		t.Fatal("pending after withdraw")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coach-prices/"+id+"/approve", admin, nil); rec.Code != http.StatusConflict {
		t.Fatalf("approve withdrawn %d %s", rec.Code, rec.Body.String())
	}

	// T2：承認後はコーチ側から取り下げられない（承認待ちが無いので 409）。
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/standard", tc, map[string]any{"price_yen": 1200}); rec.Code != http.StatusOK {
		t.Fatalf("reapply %d %s", rec.Code, rec.Body.String())
	}
	id = pendingPriceID(t, h, admin, coach, "standard")
	approveCoachPrice(t, h, admin, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/prices/standard/withdraw", tc, nil); rec.Code != http.StatusConflict {
		t.Fatalf("withdraw approved %d %s", rec.Code, rec.Body.String())
	}
	// 対応中の依頼がないので、承認と同時に有効になっている。承認した人・日時が残る。
	if price, pending, approved := planState(t, myCoachPrices(t, h, tc), "standard"); price == nil || *price != 1200 || pending != nil || approved != nil {
		t.Fatal("approved price not applied")
	}
	var status string
	var approvedBy *uuid.UUID
	queryRowArgs(t, `SELECT status, approved_by FROM coach_price_applications WHERE id=$1`, []any{id}, &status, &approvedBy)
	if status != "applied" || approvedBy == nil {
		t.Fatalf("status=%s approved_by=%v", status, approvedBy)
	}

	// 承認されていないコーチ・知らないプランは断る。
	_, stranger := demoRequester(t, h, admin)
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/standard", stranger, map[string]any{"price_yen": 1000}); rec.Code != http.StatusForbidden {
		t.Fatalf("non coach %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/gold", tc, map[string]any{"price_yen": 1000}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown plan %d", rec.Code)
	}
	// 一般の利用者は運営の一覧・承認を使えない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/coach-prices", tc, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("admin list by coach %d", rec.Code)
	}
}

// T4・T5・S5：承認しても、承認の時点で対応中の依頼が残っている間は旧価格のまま。新規申込みも旧価格で作られ、
// 切り替えを遅らせない。対応中だった依頼がすべて終わると新価格になり、base_price_yen と報酬（70%）が新価格基準になる。
// 承認の時点と切り替わった時点で、それぞれコーチに知らせる。
func TestCoachPriceSwitchWaitsForActiveRequests(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, a1 := demoRequester(t, h, admin)
	_, a2 := demoRequester(t, h, admin)
	_, a3 := demoRequester(t, h, admin)

	// 承認前から対応中の依頼。
	before := chosenReview(t, h, a1, coach.String(), "card", false)
	payChosenReview(t, h, before, "card")
	if base := requestBasePrice(t, before); base != 1000 {
		t.Fatalf("before base %d", base)
	}

	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/standard", tc, map[string]any{"price_yen": 2500}); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	approveCoachPrice(t, h, admin, pendingPriceID(t, h, admin, coach, "standard"))
	if n := notificationCount(t, coach, "価格の申請が承認されました"); n != 1 {
		t.Fatalf("approved notifications %d", n)
	}

	// T4：対応中の依頼が残っている間は旧価格。tick を回しても切り替わらない。
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if price, _, approved := planState(t, myCoachPrices(t, h, tc), "standard"); price == nil || *price != 1000 || approved == nil || *approved != 2500 {
		t.Fatal("switched while a request was active")
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/estimate?tier=coach&event=100m&coach="+coach.String(), a2, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"price_yen":1000`) {
		t.Fatalf("estimate during wait %d %s", rec.Code, rec.Body.String())
	}
	// 切り替え待ちの間の新規申込みは旧価格で作られる。
	during := chosenReview(t, h, a2, coach.String(), "card", false)
	if base := requestBasePrice(t, during); base != 1000 {
		t.Fatalf("during base %d", base)
	}
	if form := f.lastSession(); form.Get("line_items[0][price_data][unit_amount]") != "1000" {
		t.Fatalf("during checkout %v", form)
	}
	payChosenReview(t, h, during, "card")

	// 承認の時点で対応中だった依頼を返却する。承認後に来た依頼（during）はまだ対応中だが、切り替えを遅らせない。
	returnReview(t, h, tc, before)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"coach_prices_applied":1`) {
		t.Fatalf("tick after return %d %s", rec.Code, rec.Body.String())
	}
	if price, _, approved := planState(t, myCoachPrices(t, h, tc), "standard"); price == nil || *price != 2500 || approved != nil {
		t.Fatal("price did not switch after the active requests finished")
	}
	if n := notificationCount(t, coach, "新しい価格に切り替わりました"); n != 1 {
		t.Fatalf("applied notifications %d", n)
	}

	// T5：切り替え後の新規申込みは新価格。進行中の依頼は旧価格のまま完走する。
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/estimate?tier=coach&event=100m&coach="+coach.String(), a3, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"price_yen":2500`) {
		t.Fatalf("estimate after switch %d %s", rec.Code, rec.Body.String())
	}
	after := chosenReview(t, h, a3, coach.String(), "card", false)
	if base := requestBasePrice(t, after); base != 2500 {
		t.Fatalf("after base %d", base)
	}
	if form := f.lastSession(); form.Get("line_items[0][price_data][unit_amount]") != "2500" {
		t.Fatalf("after checkout %v", form)
	}
	payChosenReview(t, h, after, "card")
	if base := requestBasePrice(t, during); base != 1000 {
		t.Fatalf("during base changed %d", base)
	}
	returnReview(t, h, tc, during)
	returnReview(t, h, tc, after)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	for id, want := range map[string][2]int{after: {2500, 1750}, during: {1000, 700}, before: {1000, 700}} {
		var gross, amount int
		queryRowArgs(t, `SELECT gross_yen, amount_yen FROM coach_earnings WHERE review_request_id=$1 AND coach_id=$2`, []any{id, coach}, &gross, &amount)
		if gross != want[0] || amount != want[1] {
			t.Fatalf("earnings %s gross=%d amount=%d want %v", id, gross, amount, want)
		}
	}
	// 切り替えの知らせは1回だけ（tick を重ねても増えない）。
	if n := notificationCount(t, coach, "新しい価格に切り替わりました"); n != 1 {
		t.Fatalf("applied notifications after more ticks %d", n)
	}
}

// T6：プレミアムも独立に申請・承認・切り替えができ、スタンダードに影響しない。
// T10：一覧・詳細・指名候補の価格がコーチの有効価格に追随する。
func TestCoachPricePremiumIndependentAndPublicPrices(t *testing.T) {
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, athlete := demoRequester(t, h, admin)

	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/premium", tc, map[string]any{"price_yen": 30000}); rec.Code != http.StatusOK {
		t.Fatalf("premium apply %d %s", rec.Code, rec.Body.String())
	}
	if _, pending, _ := planState(t, myCoachPrices(t, h, tc), "standard"); pending != nil {
		t.Fatal("premium application touched standard")
	}
	approveCoachPrice(t, h, admin, pendingPriceID(t, h, admin, coach, "premium"))
	v := myCoachPrices(t, h, tc)
	if price, _, _ := planState(t, v, "premium"); price == nil || *price != 30000 {
		t.Fatalf("premium not applied %+v", v)
	}
	if price, _, _ := planState(t, v, "standard"); price == nil || *price != 1000 {
		t.Fatalf("standard changed %+v", v)
	}

	publicCoach := func() map[string]any {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/v1/coaches/"+coach.String(), "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("public coach %d %s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	if c := publicCoach(); c["review_price_yen"] != float64(1000) || c["premium_price_yen"] != float64(30000) {
		t.Fatalf("public prices %v %v", c["review_price_yen"], c["premium_price_yen"])
	}

	// スタンダードを切り替えると、詳細・一覧・指名候補が追随する。
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/prices/standard", tc, map[string]any{"price_yen": 1800}); rec.Code != http.StatusOK {
		t.Fatalf("standard apply %d %s", rec.Code, rec.Body.String())
	}
	approveCoachPrice(t, h, admin, pendingPriceID(t, h, admin, coach, "standard"))
	if c := publicCoach(); c["review_price_yen"] != float64(1800) || c["premium_price_yen"] != float64(30000) {
		t.Fatalf("public prices after switch %v %v", c["review_price_yen"], c["premium_price_yen"])
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"review_price_yen":1800`) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", athlete, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"price_yen":1800`) {
		t.Fatalf("nominees %d %s", rec.Code, rec.Body.String())
	}
}

// T7：コーチ応募フォームの希望価格は審査側で見え、価格申請の一覧に承認待ちで出る。未入力でも応募できる。
// T8：価格未設定のコーチの申込価格は従来どおり1,000円。
func TestCoachApplicationDesiredPrice(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)

	applicant := uuid.New()
	ta := token(t, applicant)
	body := coachApplicationBody()
	body["desired_standard_price_yen"] = 499
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "desired_standard_price_yen") {
		t.Fatalf("499 desired %d %s", rec.Code, rec.Body.String())
	}
	body["desired_standard_price_yen"] = 1500
	body["desired_premium_price_yen"] = 20000
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body); rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=pending&per_page=100", admin, nil)
	var list struct {
		Applications []struct {
			ProfileID               string `json:"profile_id"`
			DesiredStandardPriceYen *int   `json:"desired_standard_price_yen"`
			DesiredPremiumPriceYen  *int   `json:"desired_premium_price_yen"`
		} `json:"applications"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	found := false
	for _, a := range list.Applications {
		if a.ProfileID == applicant.String() {
			found = true
			if a.DesiredStandardPriceYen == nil || *a.DesiredStandardPriceYen != 1500 || a.DesiredPremiumPriceYen == nil || *a.DesiredPremiumPriceYen != 20000 {
				t.Fatalf("desired %+v", a)
			}
		}
	}
	if !found {
		t.Fatalf("application missing %s", rec.Body.String())
	}
	sources := map[string]string{}
	for _, a := range adminCoachPrices(t, h, admin).Applications {
		if a.CoachID == applicant.String() {
			sources[a.Plan] = a.Source + "/" + a.Status
		}
	}
	if sources["standard"] != "coach_apply/pending" || sources["premium"] != "coach_apply/pending" {
		t.Fatalf("price applications %v", sources)
	}
	// 応募の書き直しで希望価格を変えると、同じ承認待ちの1件を書き換える。
	body["desired_standard_price_yen"] = 1600
	delete(body, "desired_premium_price_yen")
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, body); rec.Code != http.StatusOK {
		t.Fatalf("reapply %d %s", rec.Code, rec.Body.String())
	}
	var n, price int
	queryRowArgs(t, `SELECT count(*), max(price_yen) FROM coach_price_applications WHERE coach_id=$1 AND plan='standard'`, []any{applicant}, &n, &price)
	if n != 1 || price != 1600 {
		t.Fatalf("standard applications n=%d price=%d", n, price)
	}

	// 希望価格を承認すると、そのコーチの初期価格になる。
	approveCoachPrice(t, h, admin, pendingPriceID(t, h, admin, applicant, "standard"))
	var effective int
	queryRowArgs(t, `SELECT price_yen FROM coach_prices WHERE coach_id=$1 AND plan='standard'`, []any{applicant}, &effective)
	if effective != 1600 {
		t.Fatalf("initial price %d", effective)
	}

	// 未入力でも応募できる。T8：価格未設定のコーチは1,000円で申し込まれる。
	plainCoach, _ := onDutyCoach(t, h, admin)
	var none int
	queryRowArgs(t, `SELECT count(*) FROM coach_price_applications WHERE coach_id=$1`, []any{plainCoach}, &none)
	if none != 0 {
		t.Fatalf("applications without desired price %d", none)
	}
	_, athlete := demoRequester(t, h, admin)
	id := chosenReview(t, h, athlete, plainCoach.String(), "card", false)
	if base := requestBasePrice(t, id); base != 1000 {
		t.Fatalf("unpriced coach base %d", base)
	}
}
