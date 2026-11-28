package httpx_test

import (
	"context"
	"encoding/json"
	"fmt"
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

// プレミアム（本体 #1053、補足 7231984993・補足2 7231991592 の再設計）のテスト。T1〜T10 は Issue のテスト細目
// （T2・T8・T9 は補足の読み替え）、G1〜G4 は補足 §5 の追加分、S1 は動画保持。

// setPremiumPrice は、コーチのプレミアム有効価格（価格申請制で有効になった価格。本体 #1052）を直接設定する。
// 申請・承認・切り替えの経路は coach_price_test.go で確かめる。
func setPremiumPrice(t *testing.T, coach uuid.UUID, yen int) {
	t.Helper()
	execSQLArgs(t, `INSERT INTO coach_prices (coach_id, plan, price_yen) VALUES ($1, 'premium', $2)
		ON CONFLICT (coach_id, plan) DO UPDATE SET price_yen = EXCLUDED.price_yen`, coach, yen)
}

// premiumGroup は、テスト用のグループ指定。kind のテンプレで n 本。
type premiumGroupSpec struct {
	kind string
	n    int
}

func g(kind string, n int) premiumGroupSpec { return premiumGroupSpec{kind: kind, n: n} }

// premiumBody は、プレミアムの申込の本文。グループごとに「意識・目的」を書き、レース全体のグループの動画にはタイム等を付ける。
// 同じテンプレのグループが重なるときはテーマ名に番号を付ける。
func premiumBody(t *testing.T, h http.Handler, tok string, coach uuid.UUID, specs ...premiumGroupSpec) map[string]any {
	t.Helper()
	groups := make([]map[string]any, 0, len(specs))
	seen := map[string]int{}
	for gi, spec := range specs {
		seen[spec.kind]++
		videos := []map[string]any{}
		for i := 0; i < spec.n; i++ {
			v := map[string]any{"video_public_id": signedCoachingVideo(t, h, tok)}
			if spec.kind == "race" {
				v["race_time"], v["wind_speed"], v["body_condition"] = fmt.Sprintf("11秒%02d", i), "+1.2", "脚が少し張っていた"
			}
			videos = append(videos, v)
		}
		group := map[string]any{"kind": spec.kind, "focus_note": fmt.Sprintf("グループ%dでは接地の位置を意識", gi+1), "videos": videos}
		if seen[spec.kind] > 1 {
			group["title"] = fmt.Sprintf("%s%d", spec.kind, seen[spec.kind])
		}
		groups = append(groups, group)
	}
	return map[string]any{
		"guardian_permission": true, "plan": "premium", "tier": "coach", "event": "100m",
		"requested_coach_id": coach, "pb": "10秒98", "goal_time": "10秒80",
		"method": "card", "groups": groups,
	}
}

func premiumGroupsOf(body map[string]any) []map[string]any {
	return body["groups"].([]map[string]any)
}

func premiumVideosOf(body map[string]any) []string {
	out := []string{}
	for _, g := range premiumGroupsOf(body) {
		for _, v := range g["videos"].([]map[string]any) {
			out = append(out, v["video_public_id"].(string))
		}
	}
	return out
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
	PremiumGroups []struct {
		Position  int     `json:"position"`
		Kind      string  `json:"kind"`
		Title     string  `json:"title"`
		FocusNote *string `json:"focus_note"`
		Videos    []struct {
			Position      int     `json:"position"`
			VideoURL      *string `json:"video_url"`
			RaceTime      *string `json:"race_time"`
			WindSpeed     *string `json:"wind_speed"`
			BodyCondition *string `json:"body_condition"`
		} `json:"videos"`
	} `json:"premium_groups"`
	Response *struct {
		Hitokoto  *string `json:"hitokoto"`
		Summary   *string `json:"summary"`
		GoodPoint *string `json:"good_point"`
		Sections  []struct {
			GroupPosition int    `json:"group_position"`
			Title         string `json:"title"`
			Body          string `json:"body"`
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

// premiumSections は、グループの位置ごとの返却の記入。
func premiumSections(positions ...int) []map[string]any {
	out := []map[string]any{}
	for _, p := range positions {
		out = append(out, map[string]any{"group_position": p, "body": fmt.Sprintf("グループ%dについて：接地が前に流れています", p)})
	}
	return out
}

func premiumStart(t *testing.T, h http.Handler, tc, id string) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
}

func expect422(t *testing.T, h http.Handler, tok string, body map[string]any, field string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tok, body)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"`+field+`"`) || !strings.Contains(rec.Body.String(), "validation_error") {
		t.Fatalf("want 422 on %s: %d %s", field, rec.Code, rec.Body.String())
	}
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

// T2〜T5・T8・G1・G3：6本まで・複数グループに分けて申し込め、グループの意識・目的とレースの入力・PB・目標タイムが保存され、
// 選手とコーチの詳細でグループごとに見える。7本・0本・レース以外のタイム等・連絡先・同じテーマ名は弾く。
func TestPremiumRequestGroupsAndInputs(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	setPremiumPrice(t, coach, 8000)
	_, tu := demoRequester(t, h, admin)

	// T2：0本・7本は弾く。
	expect422(t, h, tu, premiumBody(t, h, tu, coach), "videos")
	expect422(t, h, tu, premiumBody(t, h, tu, coach, g("practice", 4), g("race", 3)), "videos")
	// 動画のないグループは作れない。
	empty := premiumBody(t, h, tu, coach, g("race", 1), g("start", 0))
	expect422(t, h, tu, empty, "groups.1.videos")
	// T3：レース全体以外のグループの動画にはタイム等を書けない。
	wrong := premiumBody(t, h, tu, coach, g("start", 1))
	premiumGroupsOf(wrong)[0]["videos"].([]map[string]any)[0]["race_time"] = "11秒00"
	expect422(t, h, tu, wrong, "groups.0.videos.0.race_time")
	// G3：意識・目的に連絡先らしき文字列は書けない（どのグループでも）。
	contact := premiumBody(t, h, tu, coach, g("race", 1), g("other", 1))
	premiumGroupsOf(contact)[1]["focus_note"] = "詳しくは example@example.com へ"
	expect422(t, h, tu, contact, "groups.1.focus_note")
	// 返却の引用単位はテーマ名なので、同じテーマ名のグループは作れない（テンプレ名の既定でも）。
	dup := premiumBody(t, h, tu, coach, g("practice", 1), g("start", 1))
	premiumGroupsOf(dup)[1]["title"] = "練習"
	expect422(t, h, tu, dup, "groups.1.title")
	var count int
	queryRowArgs(t, `SELECT count(*) FROM review_requests WHERE plan='premium'`, nil, &count)
	if count != 0 {
		t.Fatalf("rejected requests created %d", count)
	}

	// T2・G1：6本ちょうど・5グループ（4つ以上、同じテンプレの2つ目も可）で申し込める。テーマ名は自由に付けられる。
	six := premiumBody(t, h, tu, coach, g("race", 2), g("start", 1), g("practice", 1), g("other", 1), g("practice", 1))
	premiumGroupsOf(six)[3]["title"] = "腕振りドリル"
	created := createPremium(t, h, tu, six, "pi_card_premium_groups")
	var groups, notes, rows, raceRows int
	queryRowArgs(t, `SELECT (SELECT count(*) FROM review_request_video_groups WHERE request_id=$1),
		(SELECT count(focus_note) FROM review_request_video_groups WHERE request_id=$1),
		(SELECT count(*) FROM review_request_videos WHERE request_id=$1),
		(SELECT count(*) FROM review_request_videos v JOIN review_request_video_groups g ON g.request_id=v.request_id AND g.position=v.group_position
		 WHERE v.request_id=$1 AND g.kind='race' AND v.race_time IS NOT NULL AND v.wind_speed IS NOT NULL AND v.body_condition IS NOT NULL)`,
		[]any{created.ID}, &groups, &notes, &rows, &raceRows)
	if groups != 5 || notes != 5 || rows != 6 || raceRows != 2 {
		t.Fatalf("groups=%d notes=%d rows=%d race=%d", groups, notes, rows, raceRows)
	}
	// T5：PB・目標タイムは申込全体に1回。風速・コンディション・意識していることは申込全体には持たない。
	var pb, goal string
	var wind, cond, focus *string
	queryRowArgs(t, `SELECT pb, goal_time, wind_speed, body_condition, focus_note FROM review_requests WHERE id=$1`, []any{created.ID}, &pb, &goal, &wind, &cond, &focus)
	if pb != "10秒98" || goal != "10秒80" || wind != nil || cond != nil || focus != nil {
		t.Fatalf("request inputs %s %s %v %v %v", pb, goal, wind, cond, focus)
	}

	mine := decodePremium(t, doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, tu, nil).Body.Bytes())
	assertPremiumGroups(t, mine)

	// T8：コーチの詳細（着手後）でもグループごとに表示され、意識・目的とレースの入力、PB・目標タイムが見える。
	premiumStart(t, h, tc, created.ID)
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+created.ID, tc, nil)
	theirs := decodePremium(t, rec.Body.Bytes())
	if rec.Code != 200 || theirs.Pb != "10秒98" || theirs.GoalTime != "10秒80" {
		t.Fatalf("coach view %d %s", rec.Code, rec.Body.String())
	}
	assertPremiumGroups(t, theirs)
	if theirs.PremiumGroups[0].Videos[0].VideoURL == nil {
		t.Fatal("coach cannot play premium video")
	}
	if raw := doJSON(t, h, http.MethodGet, "/v1/coach/requests", tc, nil).Body.String(); !strings.Contains(raw, `"plan":"premium"`) {
		t.Fatalf("coach list %s", raw)
	}
	if raw := doJSON(t, h, http.MethodGet, "/v1/me/coaching/requests", tu, nil).Body.String(); !strings.Contains(raw, `"plan":"premium"`) {
		t.Fatalf("my list %s", raw)
	}
}

func assertPremiumGroups(t *testing.T, v premiumView) {
	t.Helper()
	titles := []string{}
	for _, g := range v.PremiumGroups {
		titles = append(titles, g.Title)
		// G3・補足2：意識・目的は全グループにあり、どのグループでも見える。
		if g.FocusNote == nil || !strings.Contains(*g.FocusNote, "接地の位置を意識") {
			t.Fatalf("focus note on %s: %v", g.Title, g.FocusNote)
		}
		for _, item := range g.Videos {
			race := item.RaceTime != nil && item.WindSpeed != nil && item.BodyCondition != nil
			if (g.Kind == "race") != race {
				t.Fatalf("race inputs on %s: %+v", g.Kind, item)
			}
		}
	}
	// グループは選手が作った順。テーマ名の既定はテンプレ名。
	if !slices.Equal(titles, []string{"レース全体", "スタート", "練習", "腕振りドリル", "practice2"}) {
		t.Fatalf("groups %v", titles)
	}
	if v.VideoCount != 6 || len(v.PremiumGroups[0].Videos) != 2 {
		t.Fatalf("count %d race %d", v.VideoCount, len(v.PremiumGroups[0].Videos))
	}
	// 動画の通し番号は追加した順。
	if v.PremiumGroups[0].Videos[1].Position != 2 || v.PremiumGroups[4].Videos[0].Position != 6 {
		t.Fatalf("order %+v", v.PremiumGroups)
	}
}

// T6：決済額はコーチのプレミアム有効価格。クーポン併用時は割引後を払い、報酬は割引前×70%。チケットでは申し込めない。
// T9：返却（動画があるグループごとの記入＋最後のまとめ）が保存・表示され、まとめは末尾（グループの記入の後）に出る。
func TestPremiumPaymentCouponAndReturn(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, f := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	setPremiumPrice(t, coach, 8000)
	userID, tu := demoRequester(t, h, admin)

	// チケットでは申し込めない（残りがあっても）。
	buySixTickets(t, h, tu)
	tickets := premiumBody(t, h, tu, coach, g("race", 1))
	tickets["method"], tickets["use_tickets"] = "tickets", true
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, tickets); rec.Code != 422 || !strings.Contains(rec.Body.String(), `"method"`) {
		t.Fatalf("tickets %d %s", rec.Code, rec.Body.String())
	}

	coupon := grantTestCoupon(t, userID, "6 months")
	body := premiumBody(t, h, tu, coach, g("race", 1), g("practice", 2), g("start", 1))
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
	premiumStart(t, h, tc, created.ID)
	if raw := doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+created.ID, tc, nil).Body.String(); !strings.Contains(raw, `"coach_reward_yen":5600`) {
		t.Fatalf("reward %s", raw)
	}

	respond := func(body map[string]any) (int, string) {
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/response", tc, body)
		return rec.Code, rec.Body.String()
	}
	// グループ（3つ）は全部書く。足りない・無いグループに書く・まとめなし・連絡先は弾く。
	if code, raw := respond(map[string]any{"summary": "全体は良い流れです", "sections": premiumSections(1, 3)}); code != 422 || !strings.Contains(raw, `"sections.2"`) {
		t.Fatalf("missing section %d %s", code, raw)
	}
	if code, raw := respond(map[string]any{"summary": "全体は良い流れです", "sections": premiumSections(1, 2, 3, 4)}); code != 422 || !strings.Contains(raw, `"sections.4"`) {
		t.Fatalf("section without group %d %s", code, raw)
	}
	if code, raw := respond(map[string]any{"sections": premiumSections(1, 2, 3)}); code != 422 || !strings.Contains(raw, `"summary"`) {
		t.Fatalf("no summary %d %s", code, raw)
	}
	if code, raw := respond(map[string]any{"summary": "LINE: abc_123 で連絡ください", "sections": premiumSections(1, 2, 3)}); code != 422 || !strings.Contains(raw, `"summary"`) {
		t.Fatalf("contact summary %d %s", code, raw)
	}
	// 旧形の「全体の一言診断」だけではまとめにならない。スタンダードの型で送っても、プレミアムにはグループの記入が要る。
	if code, raw := respond(map[string]any{"hitokoto": "全体は良い流れです", "sections": premiumSections(1, 2, 3)}); code != 422 || !strings.Contains(raw, `"summary"`) {
		t.Fatalf("hitokoto as summary %d %s", code, raw)
	}
	if code, raw := respond(responseBody()); code != 422 || !strings.Contains(raw, `"sections.1"`) {
		t.Fatalf("standard form on premium %d %s", code, raw)
	}
	code, raw := respond(map[string]any{"summary": "全体は良い流れです。次はレースの後半を撮りましょう", "sections": premiumSections(3, 1, 2)})
	if code != 200 {
		t.Fatalf("return %d %s", code, raw)
	}
	if captured, _ := f.calls(); len(captured) != 1 || captured[0] != "pi_card_premium_coupon" {
		t.Fatalf("captured %v", captured)
	}
	if s := couponStatus(t, coupon); s != "used" {
		t.Fatalf("coupon after return %s", s)
	}
	mineRaw := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, tu, nil).Body.String()
	mine := decodePremium(t, []byte(mineRaw))
	if mine.Response == nil || mine.Response.Summary == nil || *mine.Response.Summary != "全体は良い流れです。次はレースの後半を撮りましょう" ||
		mine.Response.Hitokoto != nil || mine.Response.GoodPoint != nil {
		t.Fatalf("response %s", mineRaw)
	}
	got := []string{}
	for _, s := range mine.Response.Sections {
		got = append(got, fmt.Sprintf("%d:%s", s.GroupPosition, s.Title))
	}
	if !slices.Equal(got, []string{"1:レース全体", "2:練習", "3:スタート"}) {
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

// G2・G4：練習の動画だけ・1グループに6本全部でも申込と返却が成立する（グループごとの本数に制限はない）。
func TestPremiumPracticeOnlySingleGroup(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	setPremiumPrice(t, coach, 8000)
	_, tu := demoRequester(t, h, admin)
	body := premiumBody(t, h, tu, coach, g("practice", 6))
	premiumGroupsOf(body)[0]["title"] = "ミニハードル"
	created := createPremium(t, h, tu, body, "pi_card_premium_practice")
	premiumStart(t, h, tc, created.ID)
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/response", tc,
		map[string]any{"summary": "まとめ：リズムは良いです", "sections": premiumSections(1)})
	if rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	mine := decodePremium(t, doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+created.ID, tu, nil).Body.Bytes())
	if len(mine.PremiumGroups) != 1 || mine.PremiumGroups[0].Title != "ミニハードル" || len(mine.PremiumGroups[0].Videos) != 6 ||
		mine.Response == nil || len(mine.Response.Sections) != 1 || mine.Response.Sections[0].Title != "ミニハードル" {
		t.Fatalf("practice only %+v", mine)
	}
}

// T7：プレミアム価格のないコーチへのプレミアム申込は API で弾く（依頼も作らない）。
func TestPremiumRejectedForUnpricedCoach(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	coach, _ := onDutyCoach(t, h, admin)
	_, tu := demoRequester(t, h, admin)
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, premiumBody(t, h, tu, coach, g("race", 1)))
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"requested_coach_id"`) {
		t.Fatalf("unpriced %d %s", rec.Code, rec.Body.String())
	}
	// コーチの指定なしも弾く（プレミアムはおまかせにしない）。
	body := premiumBody(t, h, tu, coach, g("race", 1))
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
	premiumStart(t, h, tc, id)
	// スタンダードにグループの記入とまとめを送っても、回答の型の欄が要る。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, map[string]any{"summary": "全体", "sections": premiumSections(1)}); rec.Code != 422 || !strings.Contains(rec.Body.String(), `"good_point"`) {
		t.Fatalf("premium form on standard %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, responseBody()); rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	raw := doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+id, tu, nil).Body.String()
	if !strings.Contains(raw, `"plan":"standard"`) || !strings.Contains(raw, `"good_point":"1台目までの8歩が安定しています"`) ||
		strings.Contains(raw, `"premium_groups"`) || strings.Contains(raw, `"sections"`) || !strings.Contains(raw, `"summary":null`) || !strings.Contains(raw, `"base_price_yen":1000`) {
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
	body := premiumBody(t, h, tu, coach, g("race", 1), g("practice", 1))
	created := createPremium(t, h, tu, body, "pi_card_premium_retention")
	videos := premiumVideosOf(body)
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
	premiumStart(t, h, tc, created.ID)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+created.ID+"/response", tc, map[string]any{"summary": "全体", "sections": premiumSections(1, 2)}); rec.Code != 200 {
		t.Fatalf("return %d %s", rec.Code, rec.Body.String())
	}
	// 同じ動画は他の申込に使い回せない。
	reuse := premiumBody(t, h, tu, coach, g("start", 1))
	premiumGroupsOf(reuse)[0]["videos"].([]map[string]any)[0]["video_public_id"] = videos[0]
	grantUpload(t, h, tu, videos[0])
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests", tu, reuse); rec.Code != http.StatusConflict {
		t.Fatalf("reuse %d %s", rec.Code, rec.Body.String())
	}

	execSQLArgs(t, `UPDATE review_requests SET returned_at=now()-interval '13 days' WHERE id=$1`, created.ID)
	if rec := cleanupRequest(h, "storage-secret"); rec.Code != 200 {
		t.Fatalf("cleanup %d %s", rec.Code, rec.Body.String())
	}
	if got := deleted(); slices.Contains(got, videos[0]) {
		t.Fatalf("purged early %v", got)
	}
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
	if mine.PremiumGroups[0].Videos[0].VideoURL != nil {
		t.Fatal("purged video still playable")
	}
}

// migration 000199 の down・up が通る。down ではプレミアムの行が消え、スタンダードは残る。
// up は 000195 の形の行（枠ごと）をグループへ移し、練習の目的はグループの意識・目的に、一言診断はまとめに移す。
func TestPremiumGroupsMigrationRoundTrip(t *testing.T) {
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
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	run := func(direction string) {
		t.Helper()
		source, err := os.ReadFile("../../migrations/000199_premium_groups." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		exec(string(source))
	}
	requester, coach := uuid.New(), uuid.New()
	standard, premium := uuid.New(), uuid.New()
	exec(`INSERT INTO profiles (id, nickname) VALUES ($1, 'p'), ($2, 'c')`, requester, coach)
	insertRequests := func() {
		exec(`INSERT INTO review_requests (id, requester_id, tier, event, focus, video_public_id, status, due_at, coach_id, returned_at, plan)
			VALUES ($1, $3, 'coach', '100m', ARRAY['start'], $4, 'returned', now(), $5, now(), DEFAULT),
			       ($2, $3, 'coach', '100m', '{}', NULL, 'returned', now(), $5, now(), 'premium')
			ON CONFLICT (id) DO NOTHING`, standard, premium, requester, coachingVideoID(), coach)
	}
	insertRequests()
	exec(`INSERT INTO review_responses (request_id, coach_id, good_point, fix_points, drill) VALUES ($1, $2, '良い', ARRAY['直す'], 'ドリル')`, standard, coach)
	run("down")
	var requests, responses int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM review_requests WHERE requester_id=$1), (SELECT count(*) FROM review_responses WHERE coach_id=$2)`, requester, coach).Scan(&requests, &responses); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || responses != 1 {
		t.Fatalf("after down requests=%d responses=%d", requests, responses)
	}
	// 000195 の形の行を作ってから up で移す。
	insertRequests()
	exec(`INSERT INTO review_request_videos (request_id, position, slot, video_public_id, race_time, practice_purpose) VALUES
		($1, 1, 'practice', $2, NULL, '前傾を保つ'), ($1, 2, 'race', $3, '11秒00', NULL), ($1, 3, 'practice', $4, NULL, NULL)`,
		premium, coachingVideoID(), coachingVideoID(), coachingVideoID())
	exec(`INSERT INTO review_responses (request_id, coach_id, hitokoto) VALUES ($1, $2, '全体')`, premium, coach)
	exec(`INSERT INTO review_response_sections (request_id, slot, body) VALUES ($1, 'race', 'レースについて'), ($1, 'practice', '練習について')`, premium)
	run("up")
	rows, err := tx.Query(ctx, `SELECT g.position, g.kind, g.title, coalesce(g.focus_note, ''), count(v.*), coalesce(max(s.body), '')
		FROM review_request_video_groups g JOIN review_request_videos v ON v.request_id=g.request_id AND v.group_position=g.position
		LEFT JOIN review_response_sections s ON s.request_id=g.request_id AND s.group_position=g.position
		WHERE g.request_id=$1 GROUP BY g.position, g.kind, g.title, g.focus_note ORDER BY g.position`, premium)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for rows.Next() {
		var pos, n int
		var kind, title, note, body string
		if err := rows.Scan(&pos, &kind, &title, &note, &n, &body); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%d:%s:%s:%s:%d:%s", pos, kind, title, note, n, body))
	}
	rows.Close()
	if !slices.Equal(got, []string{"1:race:レース全体::1:レースについて", "2:practice:練習:前傾を保つ:2:練習について"}) {
		t.Fatalf("migrated %v", got)
	}
	var summary *string
	var hitokoto *string
	if err := tx.QueryRow(ctx, `SELECT summary, hitokoto FROM review_responses WHERE request_id=$1`, premium).Scan(&summary, &hitokoto); err != nil || summary == nil || *summary != "全体" || hitokoto != nil {
		t.Fatalf("summary %v hitokoto %v err %v", summary, hitokoto, err)
	}
	// 新しい行には6本の上限が効く。
	exec(`SAVEPOINT s`)
	if _, err := tx.Exec(ctx, `INSERT INTO review_request_videos (request_id, position, group_position, video_public_id) VALUES ($1, 7, 1, $2)`, premium, coachingVideoID()); err == nil {
		t.Fatal("7th video accepted")
	}
	exec(`ROLLBACK TO SAVEPOINT s`)
	var plan string
	if err := tx.QueryRow(ctx, `SELECT plan FROM review_requests WHERE id=$1`, standard).Scan(&plan); err != nil || plan != "standard" {
		t.Fatalf("plan %q %v", plan, err)
	}
}
