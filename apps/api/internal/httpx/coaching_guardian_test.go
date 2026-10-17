package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// guardianMinor は、デモ版に入った18歳未満の利用者と、保護者が同意済みのリンクのトークンを作る。
func guardianMinor(t *testing.T, h http.Handler) (string, string) {
	t.Helper()
	_, tm := demoUser(t, h, time.Date(time.Now().Year()-14, 1, 1, 0, 0, 0, 0, time.UTC))
	rec := doJSON(t, h, http.MethodPost, "/v1/me/guardian-link", tm, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("link %d %s", rec.Code, rec.Body.String())
	}
	var issued struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &issued)
	if rec := doJSON(t, h, http.MethodPost, "/v1/guardian/"+issued.Token+"/consent", "", map[string]any{
		"guardian_name": "保護者", "relationship": "mother", "agree": true,
	}); rec.Code != http.StatusOK {
		t.Fatalf("guardian consent %d %s", rec.Code, rec.Body.String())
	}
	return tm, issued.Token
}

// minorAccepted は、18歳未満の利用者が申し込み、コーチが承諾したところまで進める。
func minorAccepted(t *testing.T, h http.Handler, tm string) (string, string) {
	t.Helper()
	_, tc, planID := planCoach(t, h, 3)
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tm, contractBody(planID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	id := decodeContract(t, rec.Body.Bytes()).ID
	rec = doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+id+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if ct := decodeContract(t, rec.Body.Bytes()); ct.Status != "accepted" || ct.StartsAt != nil {
		t.Fatalf("accepted %+v", ct)
	}
	return tc, id
}

func TestCoachingGuardianParticipation(t *testing.T) {
	h := setup(t)
	tm, gt := guardianMinor(t, h)
	tc, id := minorAccepted(t, h, tm)
	gpath := "/v1/guardian/" + gt + "/contracts/" + id

	// 保護者が同意するまでは始まらず、送れない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tm, map[string]any{"body": "よろしくお願いします"}); rec.Code != http.StatusConflict {
		t.Fatalf("send before guardian %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/guardian/"+gt+"/contracts", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"accepted"`) {
		t.Fatalf("guardian list %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/guardian/"+strings.Repeat("a", 43)+"/contracts/"+id, "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("bad token %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, gpath+"/consent", "", map[string]any{"agree_report": true}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("partial consent %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, gpath+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("guardian consent %d %s", rec.Code, rec.Body.String())
	}
	if ct := decodeContract(t, rec.Body.Bytes()); ct.Status != "active" || ct.StartsAt == nil {
		t.Fatalf("started %+v", ct)
	}
	var consents int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_contract_consents WHERE contract_id = $1`, []any{id}, &consents)
	if consents != 3 {
		t.Fatalf("consents %d", consents)
	}

	// 保護者も参加者として読み書きできる。動画と連絡先は送れない。
	if rec := doJSON(t, h, http.MethodPost, gpath+"/messages", "", map[string]any{"body": "母です。よろしくお願いします。"}); rec.Code != http.StatusCreated {
		t.Fatalf("guardian send %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, gpath+"/messages", "", map[string]any{"video_public_id": "track-talk-coaching/" + uuid.NewString()}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("guardian video %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, gpath+"/messages", "", map[string]any{"body": "LINE で連絡ください"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("guardian contact %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tc, map[string]any{"body": "よろしくお願いします。"}); rec.Code != http.StatusCreated {
		t.Fatalf("coach send %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, gpath+"/messages", "", nil)
	var gview struct {
		Messages []struct {
			ID         string `json:"id"`
			SenderRole string `json:"sender_role"`
			Mine       bool   `json:"mine"`
			Active     bool   `json:"sender_active"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &gview)
	if len(gview.Messages) != 2 || !gview.Messages[0].Mine || !gview.Messages[0].Active || gview.Messages[1].Mine {
		t.Fatalf("guardian view %s", rec.Body.String())
	}
	if msgs := listMessages(t, h, tm, id); len(msgs) != 2 || msgs[0].SenderRole != "guardian" || msgs[0].Mine {
		t.Fatalf("minor view %+v", msgs)
	}
	guardianMsg, coachMsg := gview.Messages[0].ID, gview.Messages[1].ID

	// 通報。保護者は自分の書いたものを通報できない。返事に子の ID を入れない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/guardian/"+gt+"/messages/"+guardianMsg+"/report", "", map[string]any{"reason": "other"}); rec.Code != http.StatusNotFound {
		t.Fatalf("guardian self report %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/guardian/"+gt+"/messages/"+coachMsg+"/report", "", map[string]any{"reason": "harassment", "detail": "気になる言い方です"})
	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), "reporter") {
		t.Fatalf("guardian report %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/guardian/"+gt+"/messages/"+coachMsg+"/report", "", map[string]any{"reason": "other"}); rec.Code != http.StatusConflict {
		t.Fatalf("guardian report twice %d", rec.Code)
	}
	report := map[string]any{"target_type": "coaching_message", "target_id": coachMsg, "reason": "other", "detail": ""}
	if rec := doJSON(t, h, http.MethodPost, "/v1/reports", tc, report); rec.Code != http.StatusNotFound {
		t.Fatalf("coach reports own %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/reports", token(t, uuid.New()), report); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider report %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/reports", tm, report); rec.Code != http.StatusCreated {
		t.Fatalf("minor report %d %s", rec.Code, rec.Body.String())
	}
}

// 保護者が同意しなかったものは始まらない。72時間たったものは閉じる。
func TestCoachingGuardianDeclineAndExpire(t *testing.T) {
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	tm, gt := guardianMinor(t, h)
	_, declined := minorAccepted(t, h, tm)
	rec := doJSON(t, h, http.MethodPost, "/v1/guardian/"+gt+"/contracts/"+declined+"/decline", "", nil)
	if rec.Code != http.StatusOK || decodeContract(t, rec.Body.Bytes()).Status != "guardian_declined" {
		t.Fatalf("guardian decline %d %s", rec.Code, rec.Body.String())
	}
	_, late := minorAccepted(t, h, tm)
	execSQLArgs(t, `UPDATE coaching_contracts SET start_by = now() - interval '1 minute' WHERE id = $1`, late)
	if rec := doJSON(t, h, http.MethodPost, "/v1/guardian/"+gt+"/contracts/"+late+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusConflict {
		t.Fatalf("late consent %d", rec.Code)
	}
	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
		t.Fatalf("tick %d", rec.Code)
	}
	var status string
	queryRowArgs(t, `SELECT status FROM coaching_contracts WHERE id = $1`, []any{late}, &status)
	if status != "expired" {
		t.Fatalf("late status %s", status)
	}
}

func TestCoachingAdminReview(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	path := "/v1/coaching/contracts/" + id + "/messages"
	rec := doJSON(t, h, http.MethodPost, path, tc, map[string]any{"body": "ひみつの指導内容"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("send %d", rec.Code)
	}
	var sent struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sent)
	ta := adminToken(t, h)

	// 一覧と通報の一覧には、会話の中身を出さない。
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coaching/contracts", ta, nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "ひみつ") || !strings.Contains(rec.Body.String(), `"report_count":0`) {
		t.Fatalf("admin list %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/contracts", tu, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin list %d", rec.Code)
	}

	// 通報がないと開けない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/contracts/"+id, ta, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("open without report %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/reports", tu, map[string]any{"target_type": "coaching_message", "target_id": sent.Message.ID, "reason": "other", "detail": ""})
	if rec.Code != http.StatusCreated {
		t.Fatalf("report %d %s", rec.Code, rec.Body.String())
	}
	var reportID string
	queryRowArgs(t, `SELECT id::text FROM reports WHERE target_id = $1`, []any{sent.Message.ID}, &reportID)
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/reports", ta, nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "ひみつ") || !strings.Contains(rec.Body.String(), `"coaching_contract_id":"`+id+`"`) {
		t.Fatalf("admin reports %d %s", rec.Code, rec.Body.String())
	}

	// 別の契約の通報では開けない。
	_, _, other := activeContract(t, h)
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/coaching/contracts/"+other+"?report="+reportID, ta, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("open other %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/admin/coaching/contracts/"+id+"?report="+reportID, ta, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ひみつ") || !strings.Contains(rec.Body.String(), `"access_logs":[{`) {
		t.Fatalf("open %d %s", rec.Code, rec.Body.String())
	}
	var logs int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_access_logs WHERE contract_id = $1`, []any{id}, &logs)
	if logs != 1 {
		t.Fatalf("logs %d", logs)
	}
	// 開いた記録は消せない。
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM coaching_access_logs WHERE contract_id = $1`, id); err == nil {
		t.Fatal("access log deleted")
	}
	if _, err := pool.Exec(ctx, `UPDATE coaching_access_logs SET opened_at = now() WHERE contract_id = $1`, id); err == nil {
		t.Fatal("access log updated")
	}

	// 止めると送れない。読むことはできる。再開すると止めた分だけ期間が延びる。
	var before time.Time
	queryRowArgs(t, `SELECT ends_at FROM coaching_contracts WHERE id = $1`, []any{id}, &before)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+id+"/suspend", tu, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin suspend %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+id+"/suspend", ta, nil); rec.Code != http.StatusOK {
		t.Fatalf("suspend %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"body": "どうなりましたか"}); rec.Code != http.StatusConflict {
		t.Fatalf("send while suspended %d", rec.Code)
	}
	if msgs := listMessages(t, h, tu, id); len(msgs) != 1 {
		t.Fatalf("read while suspended %d", len(msgs))
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+id+"/suspend", ta, nil); rec.Code != http.StatusConflict {
		t.Fatalf("suspend twice %d", rec.Code)
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET suspended_at = now() - interval '2 days' WHERE id = $1`, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/contracts/"+id+"/resume", ta, nil); rec.Code != http.StatusOK {
		t.Fatalf("resume %d %s", rec.Code, rec.Body.String())
	}
	var after time.Time
	queryRowArgs(t, `SELECT ends_at FROM coaching_contracts WHERE id = $1`, []any{id}, &after)
	if d := after.Sub(before); d < 47*time.Hour || d > 49*time.Hour {
		t.Fatalf("extended by %v", d)
	}
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"body": "再開ありがとうございます"}); rec.Code != http.StatusCreated {
		t.Fatalf("send after resume %d", rec.Code)
	}
}
