package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

// planCoach は、誓約をしてプランを1つ公開した承認済みのコーチを作る。プランの ID を返す。
func planCoach(t *testing.T, h http.Handler, capacity int) (uuid.UUID, string, string) {
	t.Helper()
	coach := uuid.New()
	tc := token(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/applications", tc, coachApplicationBody()); rec.Code != http.StatusOK {
		t.Fatalf("apply %d", rec.Code)
	}
	withCoachPhoto(t, coach)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coach-applications/"+coach.String()+"/approve", adminToken(t, h), nil); rec.Code != http.StatusOK {
		t.Fatalf("approve %d", rec.Code)
	}
	execSQLArgs(t, `UPDATE coach_profiles SET stripe_account_id = $2, payouts_enabled = true WHERE profile_id = $1`, coach, "acct_"+coach.String())
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/pledge", tc, map[string]any{"agree": true}); rec.Code != http.StatusOK {
		t.Fatalf("pledge %d", rec.Code)
	}
	registerCommerceIdentity(t, h, tc)
	body := planBody()
	body["capacity"] = capacity
	rec := doJSON(t, h, http.MethodPost, "/v1/coach/me/plans", tc, body)
	var out struct {
		Plan struct {
			ID string `json:"id"`
		} `json:"plan"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != http.StatusCreated {
		t.Fatalf("plan %d %s", rec.Code, rec.Body.String())
	}
	return coach, tc, out.Plan.ID
}

// demoUser は、デモ版に入った利用者を作る。birth は生まれた月。
func demoUser(t *testing.T, h http.Handler, birth time.Time) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	tu := token(t, id)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", tu, nil); rec.Code != http.StatusOK {
		t.Fatalf("me %d", rec.Code)
	}
	execSQLArgs(t, `UPDATE profiles SET coaching_demo = true, birth_month = $2 WHERE id = $1`, id, birth)
	return id, tu
}

func contractBody(planID string) map[string]any {
	return map[string]any{
		"plan_id":         planID,
		"event":           "100m",
		"goal":            "秋の大会で12秒台を出したい",
		"current_record":  "100m 13秒20",
		"message":         "スタートの出遅れが気になっています。",
		"agree_report":    true,
		"agree_detection": true,
	}
}

type contractView struct {
	ID       string  `json:"id"`
	Status   string  `json:"status"`
	Minor    bool    `json:"minor"`
	StartsAt *string `json:"starts_at"`
	EndsAt   *string `json:"ends_at"`
}

func decodeContract(t *testing.T, body []byte) contractView {
	t.Helper()
	var out struct {
		Contract contractView `json:"contract"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode %v", err)
	}
	return out.Contract
}

func TestCoachingContractFlow(t *testing.T) {
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	_, tc, planID := planCoach(t, h, 1)

	// 申し込む前に見るプランは、ログインしなくても見られる。
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/plans/"+planID, "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"full":false`) {
		t.Fatalf("public plan %d %s", rec.Code, rec.Body.String())
	}

	// デモ版に入っていないと申し込めない。
	outsider := uuid.New()
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", token(t, outsider), contractBody(planID)); rec.Code != http.StatusForbidden {
		t.Fatalf("not demo %d", rec.Code)
	}

	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	// 同意がないと申し込めない。連絡先も書けない。
	bad := contractBody(planID)
	bad["agree_report"] = false
	bad["message"] = "LINE で話しましょう"
	bad["event"] = "javelin"
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, bad)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "agree_report") || !strings.Contains(rec.Body.String(), "連絡先") || !strings.Contains(rec.Body.String(), `"event"`) {
		t.Fatalf("bad %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	first := decodeContract(t, rec.Body.Bytes())
	if first.Status != "applied" || first.Minor {
		t.Fatalf("first %+v", first)
	}
	// 同じコーチに2つ目は出せない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID)); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate %d", rec.Code)
	}

	// コーチの一覧に届く。利用者はニックネームだけ。
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/contracts", tc, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), first.ID) || strings.Contains(rec.Body.String(), "requester_id") {
		t.Fatalf("coach list %d %s", rec.Code, rec.Body.String())
	}

	// コーチも同意しないと承諾できない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+first.ID+"/accept", tc, map[string]any{"agree_report": true}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("accept without consent %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+first.ID+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true})
	accepted := decodeContract(t, rec.Body.Bytes())
	if rec.Code != http.StatusOK || accepted.Status != "active" || accepted.StartsAt == nil || accepted.EndsAt == nil {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+first.ID+"/decline", tc, nil); rec.Code != http.StatusConflict {
		t.Fatalf("decline after accept %d", rec.Code)
	}

	// 上限の1人に届いたので、プランは満員になり、ほかの人は申し込めない。
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/plans/"+planID, "", nil)
	if !strings.Contains(rec.Body.String(), `"full":true`) {
		t.Fatalf("full %s", rec.Body.String())
	}
	_, tu2 := demoUser(t, h, time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC))
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu2, contractBody(planID)); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "plan_full") {
		t.Fatalf("apply full %d %s", rec.Code, rec.Body.String())
	}

	// 申し込んだ人とコーチだけが中身を見られる。
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+first.ID, tu2, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other view %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+first.ID, tu, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"requester"`) {
		t.Fatalf("view %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/coaching/contracts", tu, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), first.ID) {
		t.Fatalf("my list %d %s", rec.Code, rec.Body.String())
	}

	// 期間が過ぎたら、定期処理で終わる。
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at = now() - interval '1 minute' WHERE id = $1`, first.ID)
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"contracts_ended":1`) {
		t.Fatalf("tick end %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+first.ID, tu, nil)
	if decodeContract(t, rec.Body.Bytes()).Status != "ended" {
		t.Fatalf("not ended %s", rec.Body.String())
	}

	// 空いたので2人目が申し込める。返事のないまま48時間たつと閉じる。
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu2, contractBody(planID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply second %d %s", rec.Code, rec.Body.String())
	}
	second := decodeContract(t, rec.Body.Bytes())
	execSQLArgs(t, `UPDATE coaching_contracts SET respond_by = now() - interval '1 minute' WHERE id = $1`, second.ID)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+second.ID+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "期限") {
		t.Fatalf("accept late %d %s", rec.Code, rec.Body.String())
	}
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"contracts_expired":1`) {
		t.Fatalf("tick expire %s", rec.Body.String())
	}

	// 取り下げは返事待ちのうちだけ。
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu2, contractBody(planID))
	third := decodeContract(t, rec.Body.Bytes())
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+third.ID+"/cancel", tu, nil); rec.Code != http.StatusConflict {
		t.Fatalf("cancel by other %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+third.ID+"/cancel", tu2, nil)
	if rec.Code != http.StatusOK || decodeContract(t, rec.Body.Bytes()).Status != "cancelled" {
		t.Fatalf("cancel %d %s", rec.Code, rec.Body.String())
	}
}

func TestMinorCoachingContractAppliesWithoutGuardian(t *testing.T) {
	h := setup(t)
	_, _, planID := planCoach(t, h, 3)
	birth := time.Date(time.Now().Year()-15, 4, 1, 0, 0, 0, 0, time.UTC)
	_, tm := demoUser(t, h, birth)
	// 申し込みの時点では保護者の同意は要らない。保護者のメールは、コーチが承諾したあとに聞く。
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tm, contractBody(planID))
	if rec.Code != http.StatusCreated || !decodeContract(t, rec.Body.Bytes()).Minor {
		t.Fatalf("minor apply %d %s", rec.Code, rec.Body.String())
	}
}

// 定員1のプランで2件の申し込みを同時に承諾しても、承諾できるのは1件だけ。
func TestAcceptCoachingContractConcurrentRespectsCapacity(t *testing.T) {
	h := setup(t)
	// 書き込みの回数制限（1分に30回）に収まる回数だけ繰り返す。
	for round := 0; round < 3; round++ {
		_, tc, planID := planCoach(t, h, 1)
		var ids []string
		for i := 0; i < 2; i++ {
			_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
			rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
			if rec.Code != http.StatusCreated {
				t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
			}
			ids = append(ids, decodeContract(t, rec.Body.Bytes()).ID)
		}
		codes := make([]int, len(ids))
		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i] = doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+id+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true}).Code
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
			t.Fatalf("round %d: accepted %d of capacity 1 (%v)", round, ok, codes)
		}
	}
}
