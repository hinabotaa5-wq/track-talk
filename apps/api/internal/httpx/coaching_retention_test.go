package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestCoachingMessagesPurgedAfterOneYear(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setupConfig(t, func(cfg *config.Config) { cfg.CoachingTickSecret = "tick-secret" })
	ta := adminToken(t, h)

	send := func(tok, id string, body map[string]any) string {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/messages", tok, body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("send %d %s", rec.Code, rec.Body.String())
		}
		var out struct {
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out.Message.ID
	}

	// 1年たった契約、通報が残っている契約、まだ1年たっていない契約。
	tc, tu, old := activeContract(t, h)
	send(tu, old, map[string]any{"body": "よろしくお願いします"})
	send(tu, old, map[string]any{"video_public_id": signedCoachingVideo(t, h, tu)})
	send(tc, old, map[string]any{"body": "1週目のメニューです"})
	tc2, tu2, reported := activeContract(t, h)
	msg := send(tc2, reported, map[string]any{"body": "通報するメッセージ"})
	if rec := doJSON(t, h, http.MethodPost, "/v1/reports", tu2, map[string]any{"target_type": "coaching_message", "target_id": msg, "reason": "other", "detail": ""}); rec.Code != http.StatusCreated {
		t.Fatalf("report %d %s", rec.Code, rec.Body.String())
	}
	_, tu3, recent := activeContract(t, h)
	send(tu3, recent, map[string]any{"body": "まだ読める"})

	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'ended', starts_at = now() - interval '14 months', ends_at = now() - interval '13 months' WHERE id = ANY($1::uuid[])`, []string{old, reported})
	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'ended', starts_at = now() - interval '12 months', ends_at = now() - interval '11 months' WHERE id = $1`, recent)

	if rec := tick(t, h, "tick-secret"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"messages_purged":1`) {
		t.Fatalf("tick %d %s", rec.Code, rec.Body.String())
	}
	if got := listMessages(t, h, tu, old); len(got) != 0 {
		t.Fatalf("old messages %+v", got)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+old, tu, nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"messages_purged_at":null`) || !strings.Contains(rec.Body.String(), `"messages_purged_at":"`) {
		t.Fatalf("old contract %d %s", rec.Code, rec.Body.String())
	}
	if got := listMessages(t, h, tu2, reported); len(got) != 1 {
		t.Fatalf("reported kept %+v", got)
	}
	if got := listMessages(t, h, tu3, recent); len(got) != 1 {
		t.Fatalf("recent kept %+v", got)
	}

	// 運営が通報の対応を終えたら、次の tick で消す。
	var reportID string
	queryRowArgs(t, `SELECT id::text FROM reports WHERE target_id = $1`, []any{msg}, &reportID)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/reports/"+reportID+"/close", tu2, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin close %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/reports/"+reportID+"/close", ta, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"reviewed"`) {
		t.Fatalf("close %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/reports/"+reportID+"/close", ta, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("close again %d", rec.Code)
	}
	if rec := tick(t, h, "tick-secret"); !strings.Contains(rec.Body.String(), `"messages_purged":1`) {
		t.Fatalf("tick after close %s", rec.Body.String())
	}
	if got := listMessages(t, h, tu2, reported); len(got) != 0 {
		t.Fatalf("reported after close %+v", got)
	}
	// 契約そのものは残る。
	if contractStatus(t, reported) != "ended" {
		t.Fatalf("contract status %s", contractStatus(t, reported))
	}
}
