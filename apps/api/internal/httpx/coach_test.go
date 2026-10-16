package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type coachApplication struct {
	Tier         string   `json:"tier"`
	EventGroups  []string `json:"event_groups"`
	BestRecord   string   `json:"best_record"`
	Achievements string   `json:"achievements"`
	Years        int      `json:"years"`
	Status       string   `json:"status"`
	ReviewedAt   *string  `json:"reviewed_at"`
}

type coachMeView struct {
	Application *coachApplication `json:"application"`
}

func coachApplicationBody() map[string]any {
	return map[string]any{
		"tier":         "senpai",
		"event_groups": []string{"hurdles", "sprint", "sprint"},
		"best_record":  "100m 10秒95（2025年）",
		"achievements": "インターハイ 100m 準決勝",
		"years":        8,
	}
}

func TestCoachApplicationFlow(t *testing.T) {
	h := setup(t)
	applicant := uuid.New()
	ta := token(t, applicant)

	rec := doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"application":null`) {
		t.Fatalf("fresh %d %s", rec.Code, rec.Body.String())
	}

	bad := coachApplicationBody()
	bad["tier"] = "pro"
	bad["event_groups"] = []string{"relay"}
	bad["best_record"] = " "
	bad["years"] = 61
	delete(bad, "achievements")
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, bad)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid %d %s", rec.Code, rec.Body.String())
	}
	for _, field := range []string{"tier", "event_groups", "best_record", "achievements", "years"} {
		if !strings.Contains(rec.Body.String(), `"`+field+`"`) {
			t.Fatalf("missing field error %s: %s", field, rec.Body.String())
		}
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var me coachMeView
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Application == nil || me.Application.Status != "pending" || strings.Join(me.Application.EventGroups, ",") != "sprint,hurdles" {
		t.Fatalf("applied %+v", me.Application)
	}

	// 審査待ちのうちは書き直せる。
	edit := coachApplicationBody()
	edit["years"] = 9
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, edit)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"years":9`) {
		t.Fatalf("edit %d %s", rec.Code, rec.Body.String())
	}

	// 管理者でなければ一覧も審査もできない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications", ta, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin list %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", ta, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin approve %d", rec.Code)
	}

	admin := uuid.New()
	tadmin := token(t, admin)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tadmin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin me %d", rec.Code)
	}
	setAdmin(t, admin)

	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), applicant.String()) {
		t.Fatalf("admin list %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=unknown", tadmin, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad status %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+uuid.NewString()+"/approve", tadmin, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing %d", rec.Code)
	}

	// 却下されたら出し直せて、また審査待ちに戻る。
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/reject", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"rejected"`) {
		t.Fatalf("reject %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil); rec.Code != http.StatusConflict {
		t.Fatalf("approve rejected %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, coachApplicationBody())
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if rec.Code != http.StatusOK || me.Application.Status != "pending" || me.Application.ReviewedAt != nil {
		t.Fatalf("reapply %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+applicant.String()+"/approve", tadmin, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"approved"`) {
		t.Fatalf("approve %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coach-applications?status=approved", tadmin, nil); !strings.Contains(rec.Body.String(), applicant.String()) {
		t.Fatalf("approved list %s", rec.Body.String())
	}

	// 承認された後は応募内容を変えられない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", ta, edit); rec.Code != http.StatusConflict {
		t.Fatalf("edit approved %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/me", ta, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &me)
	if me.Application == nil || me.Application.Status != "approved" || me.Application.Years != 8 {
		t.Fatalf("me after approve %+v", me.Application)
	}
}
