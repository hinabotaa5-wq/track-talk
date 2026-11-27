package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// コーチの添削項目（追加項目）と見本動画の変更申請（本体 #1086）のテスト。A1〜A9 は Issue の番号。

type coachOfferAppView struct {
	ID         string   `json:"id"`
	Items      []string `json:"items"`
	DemoVideos []struct {
		PublicID string `json:"public_id"`
		URL      string `json:"url"`
	} `json:"demo_videos"`
	Status string `json:"status"`
}

type coachOffersView struct {
	Items      []string `json:"items"`
	DemoVideos []struct {
		PublicID string `json:"public_id"`
		URL      string `json:"url"`
	} `json:"demo_videos"`
	Pending  *coachOfferAppView `json:"pending"`
	Approved *coachOfferAppView `json:"approved"`
}

type adminCoachOffersView struct {
	Applications []struct {
		ID         string   `json:"id"`
		CoachID    string   `json:"coach_id"`
		Items      []string `json:"items"`
		DemoVideos []struct {
			URL string `json:"url"`
		} `json:"demo_videos"`
		CurrentItems      []string `json:"current_items"`
		CurrentDemoVideos []struct {
			URL string `json:"url"`
		} `json:"current_demo_videos"`
		Status             string  `json:"status"`
		ApprovedAt         *string `json:"approved_at"`
		ApprovedByNickname *string `json:"approved_by_nickname"`
	} `json:"applications"`
}

func myCoachOffers(t *testing.T, h http.Handler, tc string) coachOffersView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coach/me/offers", tc, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("my offers %d %s", rec.Code, rec.Body.String())
	}
	var out coachOffersView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func adminCoachOffers(t *testing.T, h http.Handler, admin string) adminCoachOffersView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/coach-offers", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin offers %d %s", rec.Code, rec.Body.String())
	}
	var out adminCoachOffersView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func applyCoachOffer(t *testing.T, h http.Handler, tc string, items, videos []string) {
	t.Helper()
	if videos == nil {
		videos = []string{}
	}
	rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/offers", tc, map[string]any{"items": items, "demo_video_public_ids": videos})
	if rec.Code != http.StatusOK {
		t.Fatalf("apply offer %d %s", rec.Code, rec.Body.String())
	}
}

func pendingOfferID(t *testing.T, h http.Handler, admin string, coach uuid.UUID) string {
	t.Helper()
	for _, a := range adminCoachOffers(t, h, admin).Applications {
		if a.CoachID == coach.String() && a.Status == "pending" {
			return a.ID
		}
	}
	t.Fatalf("no pending offer for %s", coach)
	return ""
}

func approveCoachOffer(t *testing.T, h http.Handler, admin, id string) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coach-offers/"+id+"/approve", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("approve offer %d %s", rec.Code, rec.Body.String())
	}
}

func requestOfferItems(t *testing.T, id string) []string {
	t.Helper()
	var items []string
	queryRowArgs(t, `SELECT offer_items FROM review_requests WHERE id=$1`, []any{id}, &items)
	return items
}

func publicCoachJSON(t *testing.T, h http.Handler, coach uuid.UUID) map[string]any {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaches/"+coach.String(), "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public coach %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// A1・A6・A9：申請・運営の一覧（変更前→変更後）・上書き・取り下げ。見本動画は本人のアップロードだけ。
func TestCoachOfferApplyOverwriteWithdraw(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, other := demoRequester(t, h, admin)

	v := myCoachOffers(t, h, tc)
	if len(v.Items) != 0 || len(v.DemoVideos) != 0 || v.Pending != nil || v.Approved != nil {
		t.Fatalf("initial %+v", v)
	}
	mine := signedCoachingVideo(t, h, tc)
	applyCoachOffer(t, h, tc, []string{"スタート局面の詳しい分析"}, []string{mine})
	v = myCoachOffers(t, h, tc)
	if v.Pending == nil || len(v.Pending.Items) != 1 || len(v.Pending.DemoVideos) != 1 || v.Pending.DemoVideos[0].PublicID != mine {
		t.Fatalf("pending %+v", v.Pending)
	}
	if len(v.Items) != 0 {
		t.Fatal("pending set took effect before approval")
	}

	// 上書き：承認待ちは1件のまま、セットが入れ替わる。
	second := signedCoachingVideo(t, h, tc)
	applyCoachOffer(t, h, tc, []string{"接地の分析", "腕振りの分析"}, []string{second, mine})
	var pendings int
	queryRowArgs(t, `SELECT count(*) FROM coach_offer_applications WHERE coach_id=$1 AND status='pending'`, []any{coach}, &pendings)
	if pendings != 1 {
		t.Fatalf("pendings %d", pendings)
	}
	v = myCoachOffers(t, h, tc)
	if strings.Join(v.Pending.Items, ",") != "接地の分析,腕振りの分析" || len(v.Pending.DemoVideos) != 2 || v.Pending.DemoVideos[0].PublicID != second {
		t.Fatalf("overwrite %+v", v.Pending)
	}
	found := false
	for _, a := range adminCoachOffers(t, h, admin).Applications {
		if a.CoachID == coach.String() && a.Status == "pending" {
			found = true
			// 承認の前に中身（変更後）と変更前が見える（補足 S3）。
			if len(a.Items) != 2 || len(a.DemoVideos) != 2 || a.DemoVideos[0].URL == "" || len(a.CurrentItems) != 0 {
				t.Fatalf("admin row %+v", a)
			}
		}
	}
	if !found {
		t.Fatal("pending not in admin list")
	}

	// 取り下げ。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/offers/withdraw", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	if v := myCoachOffers(t, h, tc); v.Pending != nil {
		t.Fatal("withdrawn still pending")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/offers/withdraw", tc, nil); rec.Code != http.StatusConflict {
		t.Fatalf("withdraw again %d", rec.Code)
	}

	// A6：他人がアップロードした動画は登録できない。
	foreign := signedCoachingVideo(t, h, other)
	rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/offers", tc, map[string]any{"items": []string{}, "demo_video_public_ids": []string{foreign}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "demo_video_public_ids") {
		t.Fatalf("foreign video %d %s", rec.Code, rec.Body.String())
	}

	// 承認すると承認者・日時が残り、対応中の依頼がないのですぐ有効になる。承認時と切り替え時に別文言で知らせる（A8）。
	applyCoachOffer(t, h, tc, []string{"スタート局面の詳しい分析"}, []string{mine})
	id := pendingOfferID(t, h, admin, coach)
	approveCoachOffer(t, h, admin, id)
	var status string
	var approvedBy *uuid.UUID
	queryRowArgs(t, `SELECT status, approved_by FROM coach_offer_applications WHERE id=$1`, []any{id}, &status, &approvedBy)
	if status != "applied" || approvedBy == nil {
		t.Fatalf("status %s approved_by %v", status, approvedBy)
	}
	v = myCoachOffers(t, h, tc)
	if strings.Join(v.Items, ",") != "スタート局面の詳しい分析" || len(v.DemoVideos) != 1 || v.Pending != nil || v.Approved != nil {
		t.Fatalf("after approve %+v", v)
	}
	if n := notificationCount(t, coach, "添削項目・見本動画の申請が承認されました"); n != 1 {
		t.Fatalf("approved notifications %d", n)
	}
	if n := notificationCount(t, coach, "新しい添削項目・見本動画に切り替わりました"); n != 1 {
		t.Fatalf("applied notifications %d", n)
	}
	if n := notificationCount(t, coach, "価格の申請が承認されました"); n != 0 {
		t.Fatal("price notification was sent for an offer")
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/coach-offers/"+id+"/approve", admin, nil); rec.Code != http.StatusConflict {
		t.Fatalf("approve twice %d", rec.Code)
	}
	// 価格の申請とは別のレコード（価格の申請は1件もない）。
	if v := myCoachPrices(t, h, tc); v.Plans[0].Pending != nil {
		t.Fatal("offer created a price application")
	}

	// コーチ詳細に追加項目（原文）と見本動画が出る。
	c := publicCoachJSON(t, h, coach)
	items, _ := c["offer_items"].([]any)
	videos, _ := c["demo_videos"].([]any)
	if len(items) != 1 || items[0] != "スタート局面の詳しい分析" || len(videos) != 1 {
		t.Fatalf("public offers %v %v", c["offer_items"], c["demo_videos"])
	}
	if strings.Contains(publicCoachJSONRaw(t, h, coach), mine) && strings.Contains(publicCoachJSONRaw(t, h, coach), `"public_id"`) {
		t.Fatal("public detail exposes public_id")
	}

	// 見本を全部外す申請もできる。
	applyCoachOffer(t, h, tc, []string{}, []string{})
	approveCoachOffer(t, h, admin, pendingOfferID(t, h, admin, coach))
	if v := myCoachOffers(t, h, tc); len(v.Items) != 0 || len(v.DemoVideos) != 0 {
		t.Fatalf("cleared %+v", v)
	}
	if c := publicCoachJSON(t, h, coach); len(c["demo_videos"].([]any)) != 0 {
		t.Fatal("cleared videos still public")
	}
}

func publicCoachJSONRaw(t *testing.T, h http.Handler, coach uuid.UUID) string {
	t.Helper()
	return doJSON(t, h, http.MethodGet, "/v1/coaches/"+coach.String(), "", nil).Body.String()
}

// A2・A9：入力チェック。上限ちょうどは通る。
func TestCoachOfferValidation(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	_, tc := onDutyCoach(t, h, admin)
	thirty := strings.Repeat("あ", 30)
	cases := []struct {
		name   string
		items  []string
		videos int
		field  string
	}{
		{"6件", []string{"a1", "a2", "a3", "a4", "a5", "a6"}, 0, "items"},
		{"31字", []string{thirty + "い"}, 0, "items"},
		{"空", []string{" "}, 0, "items"},
		{"重複", []string{"接地", " 接地 "}, 0, "items"},
		{"固定1", []string{"良い点"}, 0, "items"},
		{"固定2", []string{"改善点"}, 0, "items"},
		{"固定3", []string{"おすすめのドリル"}, 0, "items"},
		{"固定4", []string{"コーチなら明日から何をするか"}, 0, "items"},
		{"旧例文", []string{"コーチだったら明日から何をするか"}, 0, "items"},
		{"連絡先", []string{"インスタ @coach_123 で質問"}, 0, "items"},
		{"URL", []string{"https://example.com の解説"}, 0, "items"},
		{"見本4本", []string{}, 4, "demo_video_public_ids"},
	}
	for _, c := range cases {
		videos := []string{}
		for i := 0; i < c.videos; i++ {
			videos = append(videos, signedCoachingVideo(t, h, tc))
		}
		rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/offers", tc, map[string]any{"items": c.items, "demo_video_public_ids": videos})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"`+c.field+`"`) {
			t.Fatalf("%s: %d %s", c.name, rec.Code, rec.Body.String())
		}
	}
	if v := myCoachOffers(t, h, tc); v.Pending != nil {
		t.Fatal("invalid application was saved")
	}
	// 上限ちょうど（5件・30字・3本）は通る。
	videos := []string{signedCoachingVideo(t, h, tc), signedCoachingVideo(t, h, tc), signedCoachingVideo(t, h, tc)}
	applyCoachOffer(t, h, tc, []string{thirty, "項目2", "項目3", "項目4", "項目5"}, videos)
	if v := myCoachOffers(t, h, tc); v.Pending == nil || len(v.Pending.Items) != 5 || len(v.Pending.DemoVideos) != 3 {
		t.Fatalf("limits %+v", v.Pending)
	}
	// 承認されたコーチでなければ申請できない。
	if rec := doJSON(t, h, http.MethodPut, "/v1/coach/me/offers", token(t, uuid.New()), map[string]any{"items": []string{}}); rec.Code == http.StatusOK {
		t.Fatal("non-coach applied")
	}
}

// A3・A4・A5：承認しても対応中の依頼が終わるまで切り替わらない。依頼に固定した追加項目で回答を検証する。
func TestCoachOfferSwitchSnapshotAndResponse(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, a1 := demoRequester(t, h, admin)
	_, a2 := demoRequester(t, h, admin)
	_, a3 := demoRequester(t, h, admin)

	// 承認前から対応中の依頼（追加項目なしで固定）。
	before := chosenReview(t, h, a1, coach.String(), "card", false)
	payChosenReview(t, h, before, "card")
	applyCoachOffer(t, h, tc, []string{"スタート局面の詳しい分析", "後半の失速の分析"}, nil)
	approveCoachOffer(t, h, admin, pendingOfferID(t, h, admin, coach))
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	v := myCoachOffers(t, h, tc)
	if len(v.Items) != 0 || v.Approved == nil {
		t.Fatalf("switched while a request was active %+v", v)
	}
	if n := notificationCount(t, coach, "新しい添削項目・見本動画に切り替わりました"); n != 0 {
		t.Fatal("applied notification before switch")
	}
	// 切り替え待ちの間の新規依頼は旧セット（空）で固定される。
	during := chosenReview(t, h, a2, coach.String(), "card", false)
	if items := requestOfferItems(t, during); len(items) != 0 {
		t.Fatalf("during items %v", items)
	}
	payChosenReview(t, h, during, "card")

	// 承認の時点で対応中だった依頼を返却（従来どおりの4項目だけ。A7）。
	returnReview(t, h, tc, before)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"coach_offers_applied":1`) {
		t.Fatalf("tick after return %d %s", rec.Code, rec.Body.String())
	}
	if v := myCoachOffers(t, h, tc); len(v.Items) != 2 {
		t.Fatalf("not switched %+v", v)
	}
	if n := notificationCount(t, coach, "新しい添削項目・見本動画に切り替わりました"); n != 1 {
		t.Fatalf("applied notifications %d", n)
	}

	// 切り替え後の依頼は新しいセットで固定され、申込確認用の指名候補にも同じタイトルが出る。
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/nominees?event=100m", a3, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"offer_items":["スタート局面の詳しい分析","後半の失速の分析"]`) {
		t.Fatalf("nominees %d %s", rec.Code, rec.Body.String())
	}
	after := chosenReview(t, h, a3, coach.String(), "card", false)
	if items := requestOfferItems(t, after); strings.Join(items, ",") != "スタート局面の詳しい分析,後半の失速の分析" {
		t.Fatalf("after items %v", items)
	}
	payChosenReview(t, h, after, "card")

	// A4：有効セットを変えても、作成済みの依頼の固定は変わらない。
	applyCoachOffer(t, h, tc, []string{"別の項目"}, nil)
	approveCoachOffer(t, h, admin, pendingOfferID(t, h, admin, coach))
	if items := requestOfferItems(t, after); len(items) != 2 {
		t.Fatalf("snapshot changed %v", items)
	}

	// コーチの依頼詳細に固定した追加項目が出る。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+after+"/start", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+after, tc, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"offer_items":["スタート局面の詳しい分析","後半の失速の分析"]`) {
		t.Fatalf("coach detail %d %s", rec.Code, rec.Body.String())
	}

	// A5：追加項目の本文は必須・300字・連絡先ガード。
	send := func(items []string) *http.Response {
		body := responseBody()
		body["offer_items"] = items
		return doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+after+"/response", tc, body).Result()
	}
	for name, items := range map[string][]string{
		"未入力":  {"スタートは低く出られています"},
		"空":    {"スタートは低く出られています", " "},
		"301字": {"スタートは低く出られています", strings.Repeat("あ", 301)},
		"連絡先":  {"スタートは低く出られています", "詳しくは LINE ID: coach123 まで"},
		"多すぎ":  {"a", "b", "c"},
	} {
		if res := send(items); res.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d", name, res.StatusCode)
		}
	}
	if res := send([]string{"スタートは低く出られています", strings.Repeat("あ", 300)}); res.StatusCode != http.StatusOK {
		t.Fatalf("valid response %d", res.StatusCode)
	}
	// 選手側の表示にも、固定項目の後にタイトル付きで出る。
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/requests/"+after, a3, nil)
	var got struct {
		Response struct {
			OfferItems []struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"offer_items"`
		} `json:"response"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Response.OfferItems) != 2 || got.Response.OfferItems[0].Title != "スタート局面の詳しい分析" || got.Response.OfferItems[0].Body != "スタートは低く出られています" {
		t.Fatalf("requester view %s", rec.Body.String())
	}
	// 試用期間の管理表示にも追加項目（タイトル＋本文）が出る（補足 S8）。
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coaching/probation", admin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"title":"後半の失速の分析"`) || !strings.Contains(rec.Body.String(), `"coach_nickname"`) {
		t.Fatalf("probation %d %s", rec.Code, rec.Body.String())
	}

	// A7：追加項目なしで固定された依頼（during）は従来どおり4項目だけで返せる。
	returnReview(t, h, tc, during)
}

// A9（後半）・S6：同じ動画をプロフィールの見本と回答の見本の両方に使える。保持期間の削除でプロフィールの見本は消えない。
func TestCoachOfferDemoVideoSharedWithResponse(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupCoachChoice(t)
	admin := adminToken(t, h)
	coach, tc := onDutyCoach(t, h, admin)
	_, a1 := demoRequester(t, h, admin)

	shared := signedCoachingVideo(t, h, tc)
	applyCoachOffer(t, h, tc, nil, []string{shared})
	approveCoachOffer(t, h, admin, pendingOfferID(t, h, admin, coach))

	id := chosenReview(t, h, a1, coach.String(), "card", false)
	payChosenReview(t, h, id, "card")
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/requests/"+id+"/start", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	body := responseBody()
	body["demo_video_public_id"] = shared
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/requests/"+id+"/response", tc, body); rec.Code != http.StatusOK {
		t.Fatalf("response with shared demo %d %s", rec.Code, rec.Body.String())
	}
	// 回答に使ったあとでも、プロフィールの見本として出し直せる。
	applyCoachOffer(t, h, tc, []string{"接地の分析"}, []string{shared})
	// 保持期間の削除で消す動画の一覧（回答の見本動画）から、プロフィールで使っている動画は外す。
	pool, err := pgxpool.New(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ids, err := db.New(pool).ListReviewVideoIDs(context.Background(), uuid.MustParse(id))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(ids, shared) {
		t.Fatalf("profile demo would be purged with the response %v", ids)
	}
	// プロフィールから外したあとは、従来どおり回答の見本動画として期限で消す対象に戻る。
	applyCoachOffer(t, h, tc, nil, nil)
	approveCoachOffer(t, h, admin, pendingOfferID(t, h, admin, coach))
	if ids, err = db.New(pool).ListReviewVideoIDs(context.Background(), uuid.MustParse(id)); err != nil || !slices.Contains(ids, shared) {
		t.Fatalf("response demo no longer purged %v %v", ids, err)
	}
}
