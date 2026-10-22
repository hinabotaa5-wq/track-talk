package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func trialPlanBody() map[string]any {
	body := planBody()
	body["kind"] = "trial"
	body["title"] = "お試し: 動画1本を見て返します"
	body["duration_days"] = 7
	body["price_yen"] = 2500
	return body
}

// お試しプランは7日・2,000〜3,000円で、1人のコーチに1つまで。
func TestTrialCoachingPlanRules(t *testing.T) {
	h := setup(t)
	_, tc, periodID := planCoach(t, h, 3)

	bad := trialPlanBody()
	bad["duration_days"] = 30
	bad["price_yen"] = 3001
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, bad)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "お試しは7日") || !strings.Contains(rec.Body.String(), "2,000〜3,000円") {
		t.Fatalf("bad trial %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, trialPlanBody())
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"kind":"trial"`) {
		t.Fatalf("trial %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Plan struct {
			ID string `json:"id"`
		} `json:"plan"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// 2つ目は作れないし、ほかのプランをお試しに直すこともできない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, trialPlanBody()); rec.Code != http.StatusConflict {
		t.Fatalf("second trial %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+periodID, tc, trialPlanBody()); rec.Code != http.StatusConflict {
		t.Fatalf("period to trial %d %s", rec.Code, rec.Body.String())
	}
	// いまのお試しプランは直せる。
	edit := trialPlanBody()
	edit["price_yen"] = 3000
	if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+created.Plan.ID, tc, edit); rec.Code != http.StatusOK {
		t.Fatalf("edit trial %d %s", rec.Code, rec.Body.String())
	}
}

// 2つのプランを同時にお試しに直しても、お試しプランは1つまでになる。
func TestTrialCoachingPlanConcurrentUpdates(t *testing.T) {
	h := setup(t)
	_, tc, first := planCoach(t, h, 3)
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, planBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("second plan %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Plan struct {
			ID string `json:"id"`
		} `json:"plan"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	ids := []string{first, created.Plan.ID}

	for round := 0; round < 5; round++ {
		codes := make([]int, len(ids))
		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i] = doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+id, tc, trialPlanBody()).Code
			}()
		}
		wg.Wait()
		ok := 0
		for _, c := range codes {
			if c == http.StatusOK {
				ok++
			}
		}
		if ok != 1 {
			t.Fatalf("round %d: お試しに直せた数 %d (%v)", round, ok, codes)
		}
		// 両方を期間プランに戻して、もう一度試す。
		for _, id := range ids {
			if rec := doJSON(t, h, http.MethodPatch, "/v1/coach/me/plans/"+id, tc, planBody()); rec.Code != http.StatusOK {
				t.Fatalf("reset %d %s", rec.Code, rec.Body.String())
			}
		}
	}
}

// お試しプランは、そのコーチの指導をはじめて受ける人だけが申し込める。
func TestTrialCoachingPlanIsForFirstTimers(t *testing.T) {
	h := setup(t)
	_, tc, periodID := planCoach(t, h, 3)
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, trialPlanBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("trial %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Plan struct {
			ID string `json:"id"`
		} `json:"plan"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	trialID := created.Plan.ID
	consent := map[string]any{"agree_report": true, "agree_detection": true}

	// はじめての人は申し込める。断られたあとなら、もう一度申し込める。
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(trialID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply trial %d %s", rec.Code, rec.Body.String())
	}
	first := decodeContract(t, rec.Body.Bytes())
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+first.ID+"/decline", tc, nil); rec.Code != http.StatusOK {
		t.Fatalf("decline %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(trialID))
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"plan_kind":"trial"`) {
		t.Fatalf("apply trial again %d %s", rec.Code, rec.Body.String())
	}
	second := decodeContract(t, rec.Body.Bytes())

	// お試しが始まって終わったら、お試しにはもう申し込めない。月額のプランには申し込める。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+second.ID+"/accept", tc, consent); rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'ended' WHERE id = $1`, second.ID)
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(trialID))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "trial_used") {
		t.Fatalf("trial used %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(periodID)); rec.Code != http.StatusCreated {
		t.Fatalf("apply period %d %s", rec.Code, rec.Body.String())
	}

	// 別の人は、はじめてなので申し込める。
	_, other := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", other, contractBody(trialID)); rec.Code != http.StatusCreated {
		t.Fatalf("other trial %d %s", rec.Code, rec.Body.String())
	}
}
