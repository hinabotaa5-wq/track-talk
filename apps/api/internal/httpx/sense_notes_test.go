package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type senseNoteStats struct {
	Total  int `json:"total"`
	Days   int `json:"days"`
	Streak int `json:"streak"`
	Today  int `json:"today"`
}

type senseNoteItem struct {
	ID         string  `json:"id"`
	Body       string  `json:"body"`
	Event      *string `json:"event"`
	EventLabel *string `json:"event_label"`
	Day        string  `json:"day"`
}

func TestSenseNotesStayWithTheWriter(t *testing.T) {
	h := setup(t)
	me := token(t, uuid.New())
	other := token(t, uuid.New())

	if rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "  "}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty body = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": strings.Repeat("あ", 301)}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("long body = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "x", "event": "marathon"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown event = %d", rec.Code)
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "接地で地面を押せた", "event": "100m"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Note  senseNoteItem  `json:"note"`
		Stats senseNoteStats `json:"stats"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Note.EventLabel == nil || *created.Note.EventLabel != "100m" || created.Note.Day == "" {
		t.Fatalf("note = %+v", created.Note)
	}
	if created.Stats != (senseNoteStats{Total: 1, Days: 1, Streak: 1, Today: 1}) {
		t.Fatalf("stats = %+v", created.Stats)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "腕振りが軽い"}); rec.Code != http.StatusCreated {
		t.Fatalf("second create = %d", rec.Code)
	}

	var list struct {
		Notes []senseNoteItem `json:"notes"`
		Stats senseNoteStats  `json:"stats"`
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Notes) != 2 || list.Notes[0].Body != "腕振りが軽い" || list.Stats.Total != 2 || list.Stats.Days != 1 {
		t.Fatalf("list = %+v", list)
	}

	// 他の人には見えず、編集・削除もできない。
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", other, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Notes) != 0 || list.Stats.Total != 0 {
		t.Fatalf("other sees = %+v", list)
	}
	path := "/v1/me/sense-notes/" + created.Note.ID
	if rec := doJSON(t, h, http.MethodPatch, path, other, map[string]any{"body": "乗っ取り"}); rec.Code != http.StatusNotFound {
		t.Fatalf("other patch = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, path, other, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other delete = %d", rec.Code)
	}
	// 公開のフィードには出ない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil); strings.Contains(rec.Body.String(), "接地で地面を押せた") {
		t.Fatal("sense note leaked into the feed")
	}

	rec = doJSON(t, h, http.MethodPatch, path, me, map[string]any{"body": "接地で地面を強く押せた", "event": nil})
	var updated senseNoteItem
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if rec.Code != http.StatusOK || updated.Body != "接地で地面を強く押せた" || updated.Event != nil {
		t.Fatalf("patch = %d %+v", rec.Code, updated)
	}

	rec = doJSON(t, h, http.MethodDelete, path, me, nil)
	var deleted struct {
		Stats senseNoteStats `json:"stats"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &deleted)
	if rec.Code != http.StatusOK || deleted.Stats.Total != 1 {
		t.Fatalf("delete = %d %+v", rec.Code, deleted)
	}
}

func TestSenseNotesPageAndDailyLimit(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	me := token(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "今日の一件目"}); rec.Code != http.StatusCreated {
		t.Fatalf("create = %d", rec.Code)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// 過去の日に1日1件ずつ35件、今日にさらに19件を入れる。
	if _, err := pool.Exec(ctx, `
		INSERT INTO sense_notes (id, author_id, body, created_at)
		SELECT gen_random_uuid(), $1, 'past', now() - make_interval(days => g)
		FROM generate_series(1, 35) AS g
	`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO sense_notes (id, author_id, body, created_at)
		SELECT gen_random_uuid(), $1, 'today', now() - make_interval(secs => g)
		FROM generate_series(1, 19) AS g
	`, id); err != nil {
		t.Fatal(err)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "21件目"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("over the daily limit = %d", rec.Code)
	}

	type page struct {
		Notes      []senseNoteItem `json:"notes"`
		NextCursor *string         `json:"next_cursor"`
		Stats      senseNoteStats  `json:"stats"`
	}
	var first, second page
	rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	if len(first.Notes) != 30 || first.NextCursor == nil || first.Notes[0].Body != "今日の一件目" {
		t.Fatalf("first page = %d notes, cursor %v", len(first.Notes), first.NextCursor)
	}
	if first.Stats.Total != 55 || first.Stats.Today != 20 || first.Stats.Days != 36 || first.Stats.Streak != 36 {
		t.Fatalf("stats = %+v", first.Stats)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes?cursor="+url.QueryEscape(*first.NextCursor), me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if len(second.Notes) != 25 || second.NextCursor != nil || second.Notes[0].ID == first.Notes[29].ID {
		t.Fatalf("second page = %d notes, cursor %v", len(second.Notes), second.NextCursor)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes?cursor=bad", me, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor = %d", rec.Code)
	}
}

// 19件ある日に同時に送っても、20件を超えて作れない。
func TestSenseNoteDailyLimitHoldsUnderConcurrency(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	me := token(t, id)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", me, nil); rec.Code != http.StatusOK {
		t.Fatalf("list %d", rec.Code)
	}
	execSQLArgs(t, `INSERT INTO sense_notes (id, author_id, body) SELECT gen_random_uuid(), $1, 'メモ' FROM generate_series(1, 19)`, id)
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "同時のメモ"})
		}()
	}
	wg.Wait()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM sense_notes WHERE author_id = $1`, []any{id}, &n)
	if n != 20 {
		t.Fatalf("notes %d", n)
	}
}
