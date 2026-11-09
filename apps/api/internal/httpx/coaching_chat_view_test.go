package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

// 契約が終わった時点で、やりとり欄は選手・コーチ・保護者のだれからも見られなくなる（運営者決定 2026-11-10、本体 #506）。
// 終わるまでのあいだに文章とメニューをダウンロードできる。通報・紛争の対応中（未対応の通報、運営の保全ホールド）は閉じない。
func TestCoachingChatClosesAtEndAndExportBefore(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tadmin := adminToken(t, h)
	tc, tu, id := activeContract(t, h)
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tu, map[string]any{"body": "よろしくお願いします"}); rec.Code != http.StatusCreated {
		t.Fatalf("send %d %s", rec.Code, rec.Body.String())
	}
	menu := map[string]any{"title": "1週目", "period": "12/1〜12/7", "note": "今週はスタートを固める", "days": []map[string]any{{"day": "月", "content": "スタートダッシュ 30m×6本（8割）"}}}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tc, map[string]any{"menu": menu}); rec.Code != http.StatusCreated {
		t.Fatalf("menu %d %s", rec.Code, rec.Body.String())
	}
	// 見直しは新しいメニューを送り直す。前のメニューは履歴として残る。
	menu["title"] = "1週目（見直し）"
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tc, map[string]any{"menu": menu}); rec.Code != http.StatusCreated {
		t.Fatalf("menu revise %d", rec.Code)
	}
	if got := listMessages(t, h, tu, id); len(got) != 3 {
		t.Fatalf("history %d", len(got))
	}
	// 選手はメニューを送れない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tu, map[string]any{"menu": menu}); rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("athlete menu %d", rec.Code)
	}

	// 期間中: 見られる期限（契約の終わり）が返り、選手とコーチがダウンロードできる。
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", tu, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"view_until":"`) || strings.Contains(rec.Body.String(), `"view_closed":true`) {
		t.Fatalf("list during contract %s", rec.Body.String())
	}
	for _, tok := range []string{tu, tc} {
		rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages/export", tok, nil)
		body := rec.Body.String()
		if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") || !strings.Contains(body, "よろしくお願いします") || !strings.Contains(body, "今週はスタートを固める") || !strings.Contains(body, "1週目（見直し）") {
			t.Fatalf("export %d %s", rec.Code, body)
		}
	}
	_, other, _ := activeContract(t, h)
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages/export", other, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other export %d", rec.Code)
	}

	// 終わる時刻を過ぎたら、終わった印が付く前（tick の前）でも閉じる。送れない。
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now()-interval '1 minute' WHERE id=$1`, id)
	for _, tok := range []string{tu, tc} {
		rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", tok, nil)
		if !strings.Contains(rec.Body.String(), `"messages":[]`) || !strings.Contains(rec.Body.String(), `"view_closed":true`) {
			t.Fatalf("after ends_at %s", rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tu, map[string]any{"body": "まだ送れる？"}); rec.Code != http.StatusConflict {
		t.Fatalf("send after end %d", rec.Code)
	}

	// 終わった直後（満了・途中終了とも）: 選手もコーチも見られない・ダウンロードできない。
	for _, status := range []string{"ended", "refunded"} {
		execSQLArgs(t, `UPDATE coaching_contracts SET status=$2, ends_at=now()-interval '1 second' WHERE id=$1`, id, status)
		for _, tok := range []string{tu, tc} {
			rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", tok, nil)
			if !strings.Contains(rec.Body.String(), `"messages":[]`) || !strings.Contains(rec.Body.String(), `"view_closed":true`) {
				t.Fatalf("%s: list after end %s", status, rec.Body.String())
			}
			if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages/export", tok, nil); rec.Code != http.StatusConflict {
				t.Fatalf("%s: export after end %d", status, rec.Code)
			}
		}
	}

	// 運営の保全ホールドがあるあいだは閉じない。外すと閉じる。
	hold := func(on bool) {
		t.Helper()
		if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/retention-hold", tadmin, map[string]any{"target": "contract", "id": id, "hold": on}); rec.Code != http.StatusOK {
			t.Fatalf("hold %v %d %s", on, rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/coaching/retention-hold", tu, map[string]any{"target": "contract", "id": id, "hold": true}); rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
		t.Fatalf("non-admin hold %d", rec.Code)
	}
	hold(true)
	if got := listMessages(t, h, tu, id); len(got) != 3 {
		t.Fatalf("held %d", len(got))
	}
	hold(false)
	if got := listMessages(t, h, tu, id); len(got) != 0 {
		t.Fatalf("released %d", len(got))
	}

	// 通報の対応中は保全して、見られるままにする。
	var msgID string
	queryRowArgs(t, `SELECT id::text FROM coaching_messages WHERE contract_id=$1 AND sender_role='coach' ORDER BY created_at LIMIT 1`, []any{id}, &msgID)
	execSQLArgs(t, `INSERT INTO reports (id, reporter_id, target_type, target_id, reason, detail) SELECT gen_random_uuid(), requester_id, 'coaching_message', $2::uuid, 'other', '' FROM coaching_contracts WHERE id=$1`, id, msgID)
	if got := listMessages(t, h, tu, id); len(got) != 3 {
		t.Fatalf("preserved during report %d", len(got))
	}
}

// 保護者も、契約が終わった時点で見られなくなる。
func TestCoachingChatClosedForGuardianAfterEnd(t *testing.T) {
	h := setup(t)
	tm, gt := guardianMinor(t, h)
	tc, id := minorAccepted(t, h, tm)
	gpath := "/v1/guardian/" + gt + "/contracts/" + id
	if rec := doJSON(t, h, http.MethodPost, gpath+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusOK {
		t.Fatalf("guardian consent %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tc, map[string]any{"body": "よろしくお願いします。"}); rec.Code != http.StatusCreated {
		t.Fatalf("coach send %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, gpath+"/messages", "", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "よろしくお願いします。") {
		t.Fatalf("guardian during contract %d %s", rec.Code, rec.Body.String())
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended', ends_at=now()-interval '1 second' WHERE id=$1`, id)
	rec := doJSON(t, h, http.MethodGet, gpath+"/messages", "", nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "よろしくお願いします。") || !strings.Contains(rec.Body.String(), `"view_closed":true`) {
		t.Fatalf("guardian after end %d %s", rec.Code, rec.Body.String())
	}
}

// 終わる3日前に、終わると見られなくなることとダウンロードを、選手とコーチに1回だけ知らせる。
func TestCoachingChatCloseNoticeBeforeEnd(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	_, _, soon := activeContract(t, h)
	_, _, later := activeContract(t, h)
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now()+interval '2 days' WHERE id=$1`, soon)
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now()+interval '4 days' WHERE id=$1`, later)
	for i := 0; i < 2; i++ {
		if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK {
			t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
		}
	}
	count := func(id string) int {
		var n int
		queryRowArgs(t, `SELECT count(*)::int FROM notifications n JOIN coaching_contracts ct ON n.recipient_id IN (ct.requester_id, ct.coach_id)
			WHERE ct.id=$1 AND n.title='パーソナル指導の期間がもうすぐ終わります' AND n.body LIKE '%ダウンロード%'`, []any{id}, &n)
		return n
	}
	if got := count(soon); got != 2 {
		t.Fatalf("soon notices %d", got)
	}
	if got := count(later); got != 0 {
		t.Fatalf("later notices %d", got)
	}
}
