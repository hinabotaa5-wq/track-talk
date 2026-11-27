package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// プレミアム（本体 #1053）のテスト。T1〜T10 は Issue のテスト細目、S1 は補足の動画保持。

// setPremiumPrice は、コーチのプレミアム有効価格（価格申請制で有効になった価格。本体 #1052）を直接設定する。
// 申請・承認・切り替えの経路は coach_price_test.go で確かめる。
func setPremiumPrice(t *testing.T, coach uuid.UUID, yen int) {
	t.Helper()
	execSQLArgs(t, `INSERT INTO coach_prices (coach_id, plan, price_yen) VALUES ($1, 'premium', $2)
		ON CONFLICT (coach_id, plan) DO UPDATE SET price_yen = EXCLUDED.price_yen`, coach, yen)
}

// premiumBody は、プレミアムの申込の本文。videos は枠の並び。レース全体・練習には入力を付ける。
func premiumBody(t *testing.T, h http.Handler, tok string, coach uuid.UUID, slots ...string) map[string]any {
	t.Helper()
	videos := make([]map[string]any, 0, len(slots))
	for i, slot := range slots {
		v := map[string]any{"slot": slot, "video_public_id": signedCoachingVideo(t, h, tok)}
		switch slot {
		case "race":
			v["race_time"], v["wind_speed"], v["body_condition"] = "11秒02", "+1.2", "脚が少し張っていた"
			if i > 0 {
				v["race_time"] = "11秒10"
			}
		case "practice":
			v["practice_purpose"] = "加速局面の前傾を保つ"
		}
		videos = append(videos, v)
	}
	return map[string]any{
		"guardian_permission": true, "plan": "premium", "tier": "coach", "event": "100m",
		"requested_coach_id": coach, "pb": "10秒98", "goal_time": "10秒80",
		"method": "card", "videos": videos,
	}
}

type premiumView struct {
	ID            string `json:"id"`
	Plan          string `json:"plan"`
	Status        string `json:"status"`
	Pb            string `json:"pb"`
	GoalTime      string `json:"goal_time"`
	BasePriceYen  int    `json:"base_price_yen"`
	CashPriceYen  int    `json:"cash_price_yen"`
	CheckoutURL   string `json:"checkout_url"`
	VideoCount    int    `json:"video_count"`
	PremiumVideos []struct {
		Slot   string `json:"slot"`
		Videos []struct {
			Position        int     `json:"position"`
			VideoURL        *string `json:"video_url"`
			RaceTime        *string `json:"race_time"`
			WindSpeed       *string `json:"wind_speed"`
			BodyCondition   *string `json:"body_condition"`
			PracticePurpose *string `json:"practice_purpose"`
		} `json:"videos"`
	} `json:"premium_videos"`
	Response *struct {
		Hitokoto  string  `json:"hitokoto"`
		GoodPoint *string `json:"good_point"`
		Sections  []struct {
			Slot string `json:"slot"`
			Body string `json:"body"`
		} `json:"sections"`
	} `json:"response"`
}

func decodePremium(t *testing.T, body []byte) premiumView {
	t.Helper()
	var v premiumView
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode %v %s", err, body)
	}
	return v
}

// createPremium は、プレミアムを申し込んで、カードの仮押さえが届いたところまで進める。
func createPremium(t *testing.T, h http.Handler, tok string, body map[string]any, intent string) premiumView {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tok, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create premium %d %s", rec.Code, rec.Body.String())
	}
	created := decodePremium(t, rec.Body.Bytes())
	if created.Plan != "premium" || created.Status != "awaiting_payment" || created.CheckoutURL == "" {
		t.Fatalf("created %s", rec.Body.String())
	}
	authorizeReview(t, h, created.ID, intent)
	return created
}

func premiumSections(slots ...string) []map[string]any {
	out := []map[string]any{}
	for _, s := range slots {
		out = append(out, map[string]any{"slot": s, "body": s + "について：接地が前に流れています"})
	}
	return out
}

// T1：プレミアム価格のあるコーチだけがプレミアムタブ（公開一覧の premium_price_yen と、プレミアムの指名候補）に出る。
func TestPremiumListedOnlyForPricedCoaches(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	admin := adminToken(t, h)
	priced, _ := onDutyCoach(t, h, admin)
	plain, _ := onDutyCoach(t, h, admin)
	setPremiumPrice(t, priced, 8000)
	_, tu := demoRequester(t, h, admin)

	rec := doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coaches %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Coaches []struct {
			ID              uuid.UUID `json:"id"`
			PremiumPriceYen *int      `json:"premium_price_yen"`
		} `json:"coaches"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	seen := map[uuid.UUID]*int{}
	for _, c := range list.Coaches {
		seen[c.ID] = c.PremiumPriceYen
	}
	if p, ok := seen[priced]; !ok || p == nil || *p != 8000 {
		t.Fatalf("priced coach %v", p)
	}
	if p, ok := seen[plain]; !ok || p != nil {
		t.Fatalf("unpriced coach %v", p)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m&plan=premium", tu, nil)
	var nominees struct {
		Coaches []struct {
			ID              uuid.UUID `json:"id"`
			PremiumPriceYen int       `json:"premium_price_yen"`
		} `json:"coaches"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &nominees)
	if rec.Code != 200 || len(nominees.Coaches) != 1 || nominees.Coaches[0].ID != priced || nominees.Coaches[0].PremiumPriceYen != 8000 {
		t.Fatalf("premium nominees %d %s", rec.Code, rec.Body.String())
	}
	// スタンダードの指名候補は従来どおり両方。
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", tu, nil)
	if !strings.Contains(rec.Body.String(), priced.String()) || !strings.Contains(rec.Body.String(), plain.String()) || !strings.Contains(rec.Body.String(), `"price_yen":1000`) {
		t.Fatalf("standard nominees %s", rec.Body.String())
	}
}

// T2〜T5・T8：複数枠・複数本で申し込め、枠ごとの入力・PB・目標タイムが保存され、選手とコーチの詳細で枠ごとに見える。
// 11本・0本・枠の違う入力は弾く。
func TestPremiumRequestSlotsAndInputs(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	setPremiumPrice(t, coach, 8000)
	_, tu := demoRequester(t, h, admin)

	// T2：0本・11本は弾く。
	empty := premiumBody(t, h, tu, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, empty); rec.Code != 422 || !strings.Contains(rec.Body.String(), `"videos"`) {
		t.Fatalf("zero videos %d %s", rec.Code, rec.Body.String())
	}
	eleven := premiumBody(t, h, tu, coach, "race", "race", "start", "start", "second_half", "second_half", "practice", "practice", "other", "other", "other")
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, eleven); rec.Code != 422 || !strings.Contains(rec.Body.String(), `"videos"`) {
		t.Fatalf("eleven videos %d %s", rec.Code, rec.Body.String())
	}
	// T3・T4：レース全体以外にタイム等、練習以外に目的は書けない。
	wrong := premiumBody(t, h, tu, coach, "start", "other")
	wrong["videos"].([]map[string]any)[0]["race_time"] = "11秒00"
	wrong["videos"].([]map[string]any)[1]["practice_purpose"] = "目的"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, wrong); rec.Code != 422 ||
		!strings.Contains(rec.Body.String(), `"videos.0.race_time"`) || !strings.Contains(rec.Body.String(), `"videos.1.practice_purpose"`) {
		t.Fatalf("wrong slot inputs %d %s", rec.Code, rec.Body.String())
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE plan='premium'`, nil, &count)
	if count != 0 {
		t.Fatalf("rejected requests created %d", count)
	}

	// T2：10本ちょうど・複数枠で申し込める。
	ten := premiumBody(t, h, tu, coach, "race", "start", "race", "second_half", "practice", "other", "start", "start", "practice", "other")
	created := createPremium(t, h, tu, ten, "pi_card_premium_slots")
	var rows, raceRows, practiceRows int
	queryRowArgs(t, `SELECT count(*), count(race_time) FILTER (WHERE slot='race' AND wind_speed IS NOT NULL AND body_condition IS NOT NULL),
		count(practice_purpose) FILTER (WHERE slot='practice') FROM review_request_videos WHERE request_id=$1`, []any{created.ID}, &rows, &raceRows, &practiceRows)
	if rows != 10 || raceRows != 2 || practiceRows != 2 {
		t.Fatalf("rows=%d race=%d practice=%d", rows, raceRows, practiceRows)
	}
	// T5：PB・目標タイムは申込全体に1回。風速・コンディションは申込全体には持たない。
	var pb, goal string
	var wind, cond *string
	queryRowArgs(t, `SELECT pb, goal_time, wind_speed, body_condition FROM review_requests WHERE id=$1`, []any{created.ID}, &pb, &goal, &wind, &cond)
	if pb != "10秒98" || goal != "10秒80" || wind != nil || cond != nil {
		t.Fatalf("request inputs %s %s %v %v", pb, goal, wind, cond)
	}

	// 選手の詳細：枠ごとのグループ（決まった順）。レース全体にだけタイム等、練習にだけ目的。
	mine := decodePremium(t, doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, tu, nil).Body.Bytes())
	assertPremiumGroups(t, mine)

	// T8：コーチの詳細（着手後）でも枠ごとにグループ表示され、入力と PB・目標タイムが見える。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+created.ID+"/start", tc, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+created.ID, tc, nil)
	theirs := decodePremium(t, rec.Body.Bytes())
	if rec.Code != 200 || theirs.Pb != "10秒98" || theirs.GoalTime != "10秒80" {
		t.Fatalf("coach view %d %s", rec.Code, rec.Body.String())
	}
	assertPremiumGroups(t, theirs)
	if theirs.PremiumVideos[0].Videos[0].VideoURL == nil {
		t.Fatal("coach cannot play premium video")
	}
	// コーチの一覧でもプレミアムと分かる。
	if raw := doJSON(t, h, http.MethodGet, "/v1/coach/requests", tc, nil).Body.String(); !strings.Contains(raw, `"plan":"premium"`) {
		t.Fatalf("coach list %s", raw)
	}
	// 選手の一覧でも。
	if raw := doJSON(t, h, http.MethodGet, "/v1/me/coaching/requests", tu, nil).Body.String(); !strings.Contains(raw, `"plan":"premium"`) {
		t.Fatalf("my list %s", raw)
	}
}

func assertPremiumGroups(t *testing.T, v premiumView) {
	t.Helper()
	slots := []string{}
	for _, g := range v.PremiumVideos {
		slots = append(slots, g.Slot)
		for _, item := range g.Videos {
			race := item.RaceTime != nil && item.WindSpeed != nil && item.BodyCondition != nil
			if (g.Slot == "race") != race {
				t.Fatalf("race inputs on %s: %+v", g.Slot, item)
			}
			if (g.Slot == "practice") != (item.PracticePurpose != nil) {
				t.Fatalf("practice purpose on %s: %+v", g.Slot, item)
			}
		}
	}
	if !slices.Equal(slots, []string{"race", "start", "second_half", "practice", "other"}) {
		t.Fatalf("groups %v", slots)
	}
	if v.VideoCount != 10 || len(v.PremiumVideos[1].Videos) != 3 {
		t.Fatalf("count %d start %d", v.VideoCount, len(v.PremiumVideos[1].Videos))
	}
	// 枠内は追加した順。
	if v.PremiumVideos[1].Videos[0].Position != 2 || v.PremiumVideos[1].Videos[2].Position != 8 {
		t.Fatalf("start order %+v", v.PremiumVideos[1].Videos)
	}
}

// T6：決済額はコーチのプレミアム有効価格。クーポン併用時は割引後を払い、報酬は割引前×70%。チケットでは申し込めない。
// T9：総括（全体の一言診断＋動画がある枠ごとのブロック）が保存・表示され、動画のない枠のブロックは出ない。
func TestPremiumPaymentCouponAndSummary(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	setPremiumPrice(t, coach, 8000)
	userID, tu := demoRequester(t, h, admin)

	// チケットでは申し込めない（残りがあっても）。
	buySixTickets(t, h, tu)
	tickets := premiumBody(t, h, tu, coach, "race")
	tickets["method"], tickets["use_tickets"] = "tickets", true
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, tickets); rec.Code != 422 || !strings.Contains(rec.Body.String(), `"method"`) {
		t.Fatalf("tickets %d %s", rec.Code, rec.Body.String())
	}

	coupon := grantTestCoupon(t, userID, "6 months")
	body := premiumBody(t, h, tu, coach, "race", "practice", "start")
	body["use_coupon"] = true
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body)
	created := decodePremium(t, rec.Body.Bytes())
	if rec.Code != http.StatusCreated || created.BasePriceYen != 8000 || created.CashPriceYen != 7500 {
		t.Fatalf("coupon create %d %s", rec.Code, rec.Body.String())
	}
	if form := f.lastSession(); form.Get("line_items[0][price_data][unit_amount]") != "7500" {
		t.Fatalf("checkout amount %v", form)
	}
	if s := couponStatus(t, coupon); s != "reserved" {
		t.Fatalf("coupon %s", s)
	}
	authorizeReview(t, h, created.ID, "pi_card_premium_coupon")
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+created.ID+"/start", tc, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	// コーチの報酬の見込みも割引前×70%。
	if raw := doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+created.ID, tc, nil).Body.String(); !strings.Contains(raw, `"coach_reward_yen":5600`) {
		t.Fatalf("reward %s", raw)
	}

	respond := func(body map[string]any) (int, string) {
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/response", tc, body)
		return rec.Code, rec.Body.String()
	}
	// 動画がある枠（レース全体・練習・スタート）は全部書く。足りない・動画のない枠に書く・一言診断なしは弾く。
	if code, raw := respond(map[string]any{"hitokoto": "全体は良い流れです", "sections": premiumSections("race", "start")}); code != 422 || !strings.Contains(raw, `"sections.practice"`) {
		t.Fatalf("missing section %d %s", code, raw)
	}
	if code, raw := respond(map[string]any{"hitokoto": "全体は良い流れです", "sections": premiumSections("race", "start", "practice", "other")}); code != 422 || !strings.Contains(raw, `"sections.other"`) {
		t.Fatalf("section without video %d %s", code, raw)
	}
	if code, raw := respond(map[string]any{"sections": premiumSections("race", "start", "practice")}); code != 422 || !strings.Contains(raw, `"hitokoto"`) {
		t.Fatalf("no hitokoto %d %s", code, raw)
	}
	// スタンダードの型で送っても、プレミアムの依頼には総括が要る。
	if code, raw := respond(responseBody()); code != 422 || !strings.Contains(raw, `"sections.race"`) {
		t.Fatalf("standard form on premium %d %s", code, raw)
	}
	code, raw := respond(map[string]any{"hitokoto": "全体は良い流れです", "sections": premiumSections("practice", "race", "start")})
	if code != 200 {
		t.Fatalf("summary %d %s", code, raw)
	}
	if captured, _ := f.calls(); len(captured) != 1 || captured[0] != "pi_card_premium_coupon" {
		t.Fatalf("captured %v", captured)
	}
	if s := couponStatus(t, coupon); s != "used" {
		t.Fatalf("coupon after return %s", s)
	}
	mine := decodePremium(t, doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, tu, nil).Body.Bytes())
	if mine.Response == nil || mine.Response.Hitokoto != "全体は良い流れです" || mine.Response.GoodPoint != nil {
		t.Fatalf("response %+v", mine.Response)
	}
	got := []string{}
	for _, s := range mine.Response.Sections {
		got = append(got, s.Slot)
	}
	if !slices.Equal(got, []string{"race", "start", "practice"}) {
		t.Fatalf("sections %v", got)
	}
	// 報酬は tick で記録する。割引前 ¥8,000 × 70% = ¥5,600。
	if rec := tick(t, h, "tick-secret"); rec.Code != 200 {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	var paid, gross, share, subsidy int
	queryRowArgs(t, `SELECT p.amount_yen, e.gross_yen, e.amount_yen, e.platform_subsidy_yen FROM coach_earnings e
		JOIN review_payments p ON p.request_id=e.review_request_id AND p.status='captured'
		WHERE e.review_request_id=$1 AND e.coach_id=$2`, []any{created.ID, coach}, &paid, &gross, &share, &subsidy)
	if paid != 7500 || gross != 8000 || share != 5600 || subsidy != 500 {
		t.Fatalf("paid=%d gross=%d earnings=%d subsidy=%d", paid, gross, share, subsidy)
	}
}

// T7：プレミアム価格のないコーチへのプレミアム申込は API で弾く（依頼も作らない）。
func TestPremiumRejectedForUnpricedCoach(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, premiumBody(t, h, tu, coach, "race"))
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"requested_coach_id"`) {
		t.Fatalf("unpriced %d %s", rec.Code, rec.Body.String())
	}
	// コーチの指定なしも弾く（プレミアムはおまかせにしない）。
	body := premiumBody(t, h, tu, coach, "race")
	delete(body, "requested_coach_id")
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body); rec.Code != 422 || !strings.Contains(rec.Body.String(), `"requested_coach_id"`) {
		t.Fatalf("no coach %d %s", rec.Code, rec.Body.String())
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE plan='premium'`, nil, &count)
	if count != 0 {
		t.Fatalf("created %d", count)
	}
}

// T10：スタンダードは従来どおり（plan は standard、動画1本・回答の型・¥1,000）。プレミアムの価格があるコーチでも変わらない。
func TestPremiumLeavesStandardUnchanged(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	setPremiumPrice(t, coach, 8000)
	_, tu := demoRequester(t, h, admin)
	id := paidReview(t, h, tu, "card", "pi_standard_regression")
	if form := f.lastSession(); form.Get("line_items[0][price_data][unit_amount]") != "1000" {
		t.Fatalf("standard checkout %v", form)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	// スタンダードに総括を送っても、回答の型の欄が要る。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, map[string]any{"hitokoto": "全体", "sections": premiumSections("race")}); rec.Code != 422 || !strings.Contains(rec.Body.String(), `"good_point"`) {
		t.Fatalf("summary on standard %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	raw := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil).Body.String()
	if !strings.Contains(raw, `"plan":"standard"`) || !strings.Contains(raw, `"good_point":"1台目までの8歩が安定しています"`) ||
		strings.Contains(raw, `"premium_videos"`) || strings.Contains(raw, `"sections"`) || !strings.Contains(raw, `"base_price_yen":1000`) {
		t.Fatalf("standard view %s", raw)
	}
	// プラン名の誤りは弾く。
	body := reviewBodyFor(t, h, tu)
	body["plan"] = "gold"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, body); rec.Code != 422 || !strings.Contains(rec.Body.String(), `"plan"`) {
		t.Fatalf("unknown plan %d %s", rec.Code, rec.Body.String())
	}
}

// S1：プレミアムの動画も、保持期限（返却から7日でスレッドが終わり、そこから7日）で消し、退会でも消す。
func TestPremiumVideosPurgedWithRetentionAndWithdrawal(t *testing.T) {
	_, deleted := retentionCloud(t)
	h, _ := setupStripeConfig(t, func(c *config.Config) { c.StorageCleanupSecret = "storage-secret" })
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	setPremiumPrice(t, coach, 8000)
	userID, tu := demoRequester(t, h, admin)
	body := premiumBody(t, h, tu, coach, "race", "practice")
	created := createPremium(t, h, tu, body, "pi_card_premium_retention")
	videos := []string{}
	for _, v := range body["videos"].([]map[string]any) {
		videos = append(videos, v["video_public_id"].(string))
	}
	// 退会で消す動画（非公開の添削動画）にも入る。
	pool, err := pgxpool.New(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	media, err := db.New(pool).ListAccountMedia(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range videos {
		if !slices.Contains(media, db.ListAccountMediaRow{Resource: "coaching", PublicID: v}) {
			t.Fatalf("withdrawal media misses %s: %+v", v, media)
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+created.ID+"/start", tc, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/response", tc, map[string]any{"hitokoto": "全体", "sections": premiumSections("race", "practice")}); rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	// 同じ動画は他の申込に使い回せない。
	reuse := premiumBody(t, h, tu, coach, "start")
	reuse["videos"].([]map[string]any)[0]["video_public_id"] = videos[0]
	grantUpload(t, h, tu, videos[0])
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reuse); rec.Code != http.StatusConflict {
		t.Fatalf("reuse %d %s", rec.Code, rec.Body.String())
	}

	// 返却から13日（スレッドの終わりから6日）ではまだ消さない。
	execSQLArgs(t, `UPDATE review_requests SET returned_at=now()-interval '13 days' WHERE id=$1`, created.ID)
	if rec := cleanupRequest(h, "storage-secret"); rec.Code != 200 {
		t.Fatalf("cleanup %d %s", rec.Code, rec.Body.String())
	}
	if got := deleted(); slices.Contains(got, videos[0]) {
		t.Fatalf("purged early %v", got)
	}
	// 15日で消す。
	execSQLArgs(t, `UPDATE review_requests SET returned_at=now()-interval '15 days' WHERE id=$1`, created.ID)
	if rec := cleanupRequest(h, "storage-secret"); rec.Code != 200 {
		t.Fatalf("cleanup %d %s", rec.Code, rec.Body.String())
	}
	got := deleted()
	for _, v := range videos {
		if !slices.Contains(got, v) {
			t.Fatalf("premium video %s not purged: %v", v, got)
		}
	}
	mine := decodePremium(t, doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, tu, nil).Body.Bytes())
	if mine.PremiumVideos[0].Videos[0].VideoURL != nil {
		t.Fatal("purged video still playable")
	}
}

// migration 000195 の down・up が通り、スタンダードの行は残る（プレミアムの行と総括は down で消える）。
func TestPremiumMigrationRoundTrip(t *testing.T) {
	_ = setup(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	run := func(direction string) {
		t.Helper()
		source, err := os.ReadFile("../../migrations/000195_premium_product." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, string(source)); err != nil {
			t.Fatalf("%s: %v", direction, err)
		}
	}
	requester, coach := uuid.New(), uuid.New()
	standard, premium := uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO profiles (id, nickname) VALUES ($1, 'p'), ($2, 'c')`, requester, coach); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO review_requests (id, requester_id, tier, event, focus, video_public_id, status, due_at, coach_id, returned_at, plan)
		VALUES ($1, $3, 'coach', '100m', ARRAY['start'], $4, 'returned', now(), $5, now(), DEFAULT),
		       ($2, $3, 'coach', '100m', '{}', NULL, 'returned', now(), $5, now(), 'premium')`, standard, premium, requester, coachingVideoID(), coach); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO review_request_videos (request_id, position, slot, video_public_id, race_time) VALUES ($1, 1, 'race', $2, '11秒00')`, premium, coachingVideoID()); err != nil {
		t.Fatal(err)
	}
	// レース全体以外にはタイムを持てない。
	if _, err := tx.Exec(ctx, `SAVEPOINT s`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO review_request_videos (request_id, position, slot, video_public_id, race_time) VALUES ($1, 2, 'start', $2, '11秒00')`, premium, coachingVideoID()); err == nil {
		t.Fatal("race_time on start accepted")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT s`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO review_responses (request_id, coach_id, good_point, fix_points, drill) VALUES ($1, $2, '良い', ARRAY['直す'], 'ドリル')`, standard, coach); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO review_responses (request_id, coach_id, hitokoto) VALUES ($1, $2, '全体')`, premium, coach); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO review_response_sections (request_id, slot, body) VALUES ($1, 'race', 'レースについて')`, premium); err != nil {
		t.Fatal(err)
	}
	run("down")
	var requests, responses int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM review_requests WHERE requester_id=$1), (SELECT count(*) FROM review_responses WHERE coach_id=$2)`, requester, coach).Scan(&requests, &responses); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || responses != 1 {
		t.Fatalf("after down requests=%d responses=%d", requests, responses)
	}
	run("up")
	var plan string
	if err := tx.QueryRow(ctx, `SELECT plan FROM review_requests WHERE id=$1`, standard).Scan(&plan); err != nil || plan != "standard" {
		t.Fatalf("plan %q %v", plan, err)
	}
}
