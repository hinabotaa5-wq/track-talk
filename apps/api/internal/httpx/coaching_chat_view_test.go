package httpx_test

import (
	"net/http"
	"strings"
	"testing"
)

// 契約が終わったあとは90日間だけ読める（送信はできない）。終了時に文章とメニューをダウンロードでき、90日を過ぎると見られない。
// 未対応の通報があるあいだは90日を過ぎても閉じない（保全）。
func TestCoachingChatViewOnlyFor90DaysAndExport(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
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

	// 終わって10日: 読めて、送れない。期限が返る。ダウンロードできる。
	execSQLArgs(t, `UPDATE coaching_contracts SET status='ended', ends_at=now()-interval '10 days' WHERE id=$1`, id)
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", tu, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"view_until":"`) || strings.Contains(rec.Body.String(), `"view_closed":true`) {
		t.Fatalf("list after end %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tu, map[string]any{"body": "まだ送れる？"}); rec.Code != http.StatusConflict {
		t.Fatalf("send after end %d", rec.Code)
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

	// 91日: 見られない・ダウンロードできない。
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at=now()-interval '91 days' WHERE id=$1`, id)
	rec = doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", tu, nil)
	if !strings.Contains(rec.Body.String(), `"messages":[]`) || !strings.Contains(rec.Body.String(), `"view_closed":true`) {
		t.Fatalf("after 90 days %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages/export", tu, nil); rec.Code != http.StatusConflict {
		t.Fatalf("export after 90 days %d", rec.Code)
	}
	// 通報の対応中は保全して、見られるままにする。
	var msgID string
	queryRowArgs(t, `SELECT id::text FROM coaching_messages WHERE contract_id=$1 AND sender_role='coach' ORDER BY created_at LIMIT 1`, []any{id}, &msgID)
	execSQLArgs(t, `INSERT INTO reports (id, reporter_id, target_type, target_id, reason, detail) SELECT gen_random_uuid(), requester_id, 'coaching_message', $2::uuid, 'other', '' FROM coaching_contracts WHERE id=$1`, id, msgID)
	if got := listMessages(t, h, tu, id); len(got) != 3 {
		t.Fatalf("preserved during report %d", len(got))
	}
}
