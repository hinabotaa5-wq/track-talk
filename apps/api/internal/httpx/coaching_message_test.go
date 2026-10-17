package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// queryRowArgs は、1行だけ返す SQL を流して dest に入れる。
func queryRowArgs(t *testing.T, sql string, args []any, dest ...any) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.QueryRow(ctx, sql, args...).Scan(dest...); err != nil {
		t.Fatal(err)
	}
}

// activeContract は、引き受けられて期間中の申し込みを作る。コーチと利用者のトークンと、申し込みの ID を返す。
func activeContract(t *testing.T, h http.Handler) (string, string, string) {
	t.Helper()
	_, tc, planID := planCoach(t, h, 3)
	_, tu := demoUser(t, h, time.Date(1995, 4, 1, 0, 0, 0, 0, time.UTC))
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts", tu, contractBody(planID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	id := decodeContract(t, rec.Body.Bytes()).ID
	if rec := doJSON(t, h, http.MethodPost, "/v1/coach/contracts/"+id+"/accept", tc, map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusOK {
		t.Fatalf("accept %d", rec.Code)
	}
	return tc, tu, id
}

type messageView struct {
	SenderRole string          `json:"sender_role"`
	Mine       bool            `json:"mine"`
	Body       string          `json:"body"`
	VideoURL   *string         `json:"video_url"`
	Menu       json.RawMessage `json:"menu"`
}

func listMessages(t *testing.T, h http.Handler, tok, id string) []messageView {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/messages", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Messages []messageView `json:"messages"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Messages
}

func TestCoachingMessages(t *testing.T) {
	fakeCoachingCloudinary(t)
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	path := "/v1/coaching/contracts/" + id + "/messages"

	// 他の人は読めない。
	if rec := doJSON(t, h, http.MethodGet, path, token(t, uuid.New()), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("outsider %d", rec.Code)
	}

	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"body": "よろしくお願いします。"}); rec.Code != http.StatusCreated {
		t.Fatalf("user send %d %s", rec.Code, rec.Body.String())
	}
	video := "track-talk-coaching/" + uuid.NewString()
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"video_public_id": video}); rec.Code != http.StatusCreated {
		t.Fatalf("video %d %s", rec.Code, rec.Body.String())
	}
	// 利用者はメニューを送れない。
	menu := map[string]any{"title": "1週目", "days": []map[string]any{{"day": "月", "content": "流し 100m×5"}}}
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"menu": menu}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("user menu %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, path, tc, map[string]any{"body": "1週目のメニューです", "menu": menu}); rec.Code != http.StatusCreated {
		t.Fatalf("coach menu %d %s", rec.Code, rec.Body.String())
	}

	// 連絡先は、どちらから送っても止まり、回数だけ数える。
	if rec := doJSON(t, h, http.MethodPost, path, tc, map[string]any{"body": "インスタで送ってください"}); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "contact_blocked") {
		t.Fatalf("coach contact %d %s", rec.Code, rec.Body.String())
	}
	bad := map[string]any{"title": "1週目", "days": []map[string]any{{"day": "月", "content": "090-1234-5678 に電話"}}}
	if rec := doJSON(t, h, http.MethodPost, path, tc, map[string]any{"menu": bad}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("menu contact %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"body": "LINE 教えてください"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("user contact %d", rec.Code)
	}
	var blocked int
	queryRowArgs(t, `SELECT coalesce(sum(count), 0)::int FROM coaching_blocked_counts WHERE contract_id = $1`, []any{id}, &blocked)
	if blocked != 3 {
		t.Fatalf("blocked %d", blocked)
	}
	var stored int
	queryRowArgs(t, `SELECT count(*)::int FROM coaching_messages WHERE contract_id = $1`, []any{id}, &stored)
	if stored != 3 {
		t.Fatalf("stored %d", stored)
	}

	msgs := listMessages(t, h, tc, id)
	if len(msgs) != 3 || msgs[0].Mine || msgs[0].SenderRole != "requester" || msgs[1].VideoURL == nil || !msgs[2].Mine || len(msgs[2].Menu) == 0 {
		t.Fatalf("coach view %+v", msgs)
	}

	// 終わったら読むだけ。利用者が1回だけ★をつけられる。
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/rating", tu, map[string]any{"stars": 5}); rec.Code != http.StatusConflict {
		t.Fatalf("rate before end %d", rec.Code)
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'ended' WHERE id = $1`, id)
	if rec := doJSON(t, h, http.MethodPost, path, tu, map[string]any{"body": "ありがとうございました"}); rec.Code != http.StatusConflict {
		t.Fatalf("send after end %d", rec.Code)
	}
	if got := listMessages(t, h, tu, id); len(got) != 3 {
		t.Fatalf("read after end %d", len(got))
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/rating", tc, map[string]any{"stars": 5}); rec.Code != http.StatusForbidden {
		t.Fatalf("coach rate %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/rating", tu, map[string]any{"stars": 4}); rec.Code != http.StatusOK {
		t.Fatalf("rate %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/rating", tu, map[string]any{"stars": 1}); rec.Code != http.StatusConflict {
		t.Fatalf("rate twice %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id, tu, nil)
	if !strings.Contains(rec.Body.String(), `"stars":4`) {
		t.Fatalf("rating in detail %s", rec.Body.String())
	}
}
