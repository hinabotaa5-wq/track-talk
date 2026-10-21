package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// プランの「何が返ってくるか」は書かなくてもよく、書いたらコーチ一覧と申し込みの写しに出る。
func TestCoachingPlanIncludes(t *testing.T) {
	h := setup(t)
	coach, tc, planID := planCoach(t, h, 3)

	// 書いていないプランは null と空で返る。
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/plans/"+planID, "", nil)
	if !strings.Contains(rec.Body.String(), `"video_reviews":null`) || !strings.Contains(rec.Body.String(), `"reply_formats":[]`) {
		t.Fatalf("unset %s", rec.Body.String())
	}

	bad := planBody()
	bad["video_reviews"] = 31
	bad["menu_updates"] = "daily"
	bad["reply_formats"] = []string{"text", "voice"}
	rec = doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, bad)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "video_reviews") ||
		!strings.Contains(rec.Body.String(), "menu_updates") || !strings.Contains(rec.Body.String(), "reply_formats") {
		t.Fatalf("bad includes %d %s", rec.Code, rec.Body.String())
	}

	body := planBody()
	body["video_reviews"] = 4
	body["menu_updates"] = "weekly"
	// 選んだ順ではなく、決まった順で持つ。
	body["reply_formats"] = []string{"video", "text"}
	rec = doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"video_reviews":4`) ||
		!strings.Contains(rec.Body.String(), `"menu_updates":"weekly"`) || !strings.Contains(rec.Body.String(), `"reply_formats":["text","video"]`) {
		t.Fatalf("set includes %d %s", rec.Code, rec.Body.String())
	}

	// コーチ一覧にも出る。
	rec = doJSON(t, h, http.MethodGet, "/v1/coaches", "", nil)
	var list struct {
		Coaches []struct {
			ID    string `json:"id"`
			Plans []struct {
				VideoReviews *int     `json:"video_reviews"`
				MenuUpdates  *string  `json:"menu_updates"`
				ReplyFormats []string `json:"reply_formats"`
			} `json:"plans"`
		} `json:"coaches"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	found := false
	for _, c := range list.Coaches {
		if c.ID == coach.String() && len(c.Plans) == 1 && c.Plans[0].VideoReviews != nil && *c.Plans[0].VideoReviews == 4 &&
			c.Plans[0].MenuUpdates != nil && *c.Plans[0].MenuUpdates == "weekly" && len(c.Plans[0].ReplyFormats) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("coach list %s", rec.Body.String())
	}

	// 申し込みには、その時点の中身が写る。あとでプランを直しても変わらない。
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"video_reviews":4`) {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	ct := decodeContract(t, rec.Body.Bytes())
	body["video_reviews"] = 0
	body["menu_updates"] = nil
	body["reply_formats"] = []string{}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+planID, tc, body); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"video_reviews":0`) {
		t.Fatalf("edit %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching/contracts", tu, nil)
	if !strings.Contains(rec.Body.String(), ct.ID) || !strings.Contains(rec.Body.String(), `"video_reviews":4`) || !strings.Contains(rec.Body.String(), `"reply_formats":["text","video"]`) {
		t.Fatalf("snapshot %s", rec.Body.String())
	}
}
