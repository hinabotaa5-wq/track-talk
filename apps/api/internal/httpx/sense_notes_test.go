package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/jst"
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
	id := uuid.New()
	me := token(t, id)
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
	// 1日に1つだけ。同じ日にもう1つは書けない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "同じ日の2つ目"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("second note of the day = %d", rec.Code)
	}
	// 書けるのはその日の分だけ。前の日や先の日は選べない。
	yesterday := time.Now().In(jst.Zone).AddDate(0, 0, -1).Format(time.DateOnly)
	for _, day := range []string{time.Now().In(jst.Zone).AddDate(0, 0, 2).Format(time.DateOnly), yesterday} {
		if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "別の日", "day": day}); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("day %s = %d", day, rec.Code)
		}
	}
	// 前の日のメモ(書けたころのもの)も一覧と連続日数に入る。
	execSQLArgs(t, `INSERT INTO sense_notes (id, author_id, body, day) VALUES (gen_random_uuid(), $1, '腕振りが軽い', $2::date)`, id, yesterday)

	var list struct {
		Notes []senseNoteItem `json:"notes"`
		Stats senseNoteStats  `json:"stats"`
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Notes) != 2 || list.Notes[0].Body != "接地で地面を押せた" || list.Stats.Total != 2 || list.Stats.Days != 2 || list.Stats.Streak != 2 {
		t.Fatalf("list = %+v", list)
	}

	// 他の人には見えず、削除もできない。
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", other, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Notes) != 0 || list.Stats.Total != 0 {
		t.Fatalf("other sees = %+v", list)
	}
	path := "/v1/me/sense-notes/" + created.Note.ID
	if rec := doJSON(t, h, http.MethodDelete, path, other, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other delete = %d", rec.Code)
	}
	// 公開のフィードには出ない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil); strings.Contains(rec.Body.String(), "接地で地面を押せた") {
		t.Fatal("sense note leaked into the feed")
	}

	// 書いたあとの感覚は直せない(書いたときの感覚をそのまま残す)。
	if rec := doJSON(t, h, http.MethodPatch, path, me, map[string]any{"body": "接地で地面を強く押せた"}); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("patch = %d, want 405", rec.Code)
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

func TestSenseNotesPage(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	me := token(t, id)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", me, map[string]any{"body": "今日の一件目"}); rec.Code != http.StatusCreated {
		t.Fatalf("create = %d", rec.Code)
	}
	// 過去の日に1日1件ずつ44件。
	execSQLArgs(t, `
		INSERT INTO sense_notes (id, author_id, body, day, created_at)
		SELECT gen_random_uuid(), $1, 'past', (timezone('Asia/Tokyo', now()))::date - g, now() - make_interval(days => g)
		FROM generate_series(1, 44) AS g
	`, id)

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
	if first.Stats.Total != 45 || first.Stats.Today != 1 || first.Stats.Days != 45 || first.Stats.Streak != 45 {
		t.Fatalf("stats = %+v", first.Stats)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes?cursor="+url.QueryEscape(*first.NextCursor), me, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if len(second.Notes) != 15 || second.NextCursor != nil || second.Notes[0].ID == first.Notes[29].ID {
		t.Fatalf("second page = %d notes, cursor %v", len(second.Notes), second.NextCursor)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes?cursor=bad", me, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor = %d", rec.Code)
	}
}

// 同じ日に同時に送っても、1つしか作れない。
func TestSenseNoteOnePerDayHoldsUnderConcurrency(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	me := token(t, id)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", me, nil); rec.Code != http.StatusOK {
		t.Fatalf("list %d", rec.Code)
	}
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
	if n != 1 {
		t.Fatalf("notes %d", n)
	}
}
