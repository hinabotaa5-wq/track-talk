package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type raceRecordsResp struct {
	Events []struct {
		Event string `json:"event"`
		Count int    `json:"count"`
	} `json:"events"`
	Records []struct {
		ID        string `json:"id"`
		RacedOn   string `json:"raced_on"`
		Event     string `json:"event"`
		Mark      int    `json:"mark"`
		Wind      *int   `json:"wind"`
		WindAided bool   `json:"wind_aided"`
		IsPB      bool   `json:"is_pb"`
		Memo      string `json:"memo"`
		MeetName  string `json:"meet_name"`
		Round     string `json:"round"`
	} `json:"records"`
}

func decodeRaceRecords(t *testing.T, body []byte) raceRecordsResp {
	t.Helper()
	var out raceRecordsResp
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRaceRecordsStayWithTheAthlete(t *testing.T) {
	h := setup(t)
	me := token(t, uuid.New())
	other := token(t, uuid.New())

	if rec := doJSON(t, h, http.MethodGet, "/v1/me/race-records", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/race-records", me, map[string]any{
		"raced_on": "2026-05-03", "event": "400m", "mark": 4950, "wind": 12,
	}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("wind on 400m = %d", rec.Code)
	}

	if rec := doJSON(t, h, http.MethodPost, "/v1/me/race-records", me, map[string]any{
		"raced_on": "2026-05-03", "event": "100m", "mark": 1112, "meet_name": strings.Repeat("あ", 61),
	}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("long meet name = %d", rec.Code)
	}

	rec := doJSON(t, h, http.MethodPost, "/v1/me/race-records", me, map[string]any{
		"raced_on": "2026-05-03", "event": "100m", "mark": 1112, "wind": 8, "memo": "スタートで出遅れた", "meet_name": "  県  選手権 ",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/me/race-records", me, map[string]any{
		"raced_on": "2026-06-10", "event": "100m", "mark": 1094, "wind": 25,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create aided = %d %s", rec.Code, rec.Body.String())
	}
	got := decodeRaceRecords(t, rec.Body.Bytes())
	if len(got.Records) != 2 || !got.Records[0].IsPB || got.Records[1].IsPB || !got.Records[1].WindAided {
		t.Fatalf("records %s", rec.Body.String())
	}
	if got.Records[0].Memo != "スタートで出遅れた" || got.Records[0].MeetName != "県 選手権" || got.Records[0].Wind == nil || *got.Records[0].Wind != 8 {
		t.Fatalf("first record %s", rec.Body.String())
	}
	id := got.Records[0].ID

	// 他人の記録は見えず、変えられず、消せない。
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/race-records", other, nil); rec.Code != http.StatusOK || len(decodeRaceRecords(t, rec.Body.Bytes()).Records) != 0 {
		t.Fatalf("other list = %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/me/race-records/"+id, other, map[string]any{
		"raced_on": "2026-05-03", "event": "100m", "mark": 999,
	}); rec.Code != http.StatusNotFound {
		t.Fatalf("other update = %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/race-records/"+id, other, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other delete = %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodPatch, "/v1/me/race-records/"+id, me, map[string]any{
		"raced_on": "2026-05-03", "event": "100m", "mark": 1102, "wind": 8,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update = %d %s", rec.Code, rec.Body.String())
	}
	if got := decodeRaceRecords(t, rec.Body.Bytes()); got.Records[0].Mark != 1102 || got.Records[0].Memo != "" || got.Records[0].MeetName != "" {
		t.Fatalf("updated %s", rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/me/race-records/"+id, me, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", rec.Code, rec.Body.String())
	}
	if got := decodeRaceRecords(t, rec.Body.Bytes()); len(got.Records) != 1 || len(got.Events) != 1 || got.Events[0].Count != 1 {
		t.Fatalf("after delete %s", rec.Body.String())
	}
}

// 999件ある状態で同時に送っても、1000件を超えて保存できない。
func TestRaceRecordLimitHoldsUnderConcurrency(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	me := token(t, id)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/race-records", me, nil); rec.Code != http.StatusOK {
		t.Fatalf("list %d", rec.Code)
	}
	execSQLArgs(t, `INSERT INTO race_records (id, author_id, raced_on, event, mark) SELECT gen_random_uuid(), $1, '2026-05-03', '100m', 1112 FROM generate_series(1, 999)`, id)
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doJSON(t, h, http.MethodPost, "/v1/me/race-records", me, map[string]any{"raced_on": "2026-05-03", "event": "100m", "mark": 1100})
		}()
	}
	wg.Wait()
	var n int
	queryRowArgs(t, `SELECT count(*) FROM race_records WHERE author_id = $1`, []any{id}, &n)
	if n != 1000 {
		t.Fatalf("records %d", n)
	}
}

func TestRaceRecordRoundsAndOrdinaryMeets(t *testing.T) {
	h := setup(t)
	me := token(t, uuid.New())
	for _, round := range []string{"決勝", "予選", "準決勝", ""} {
		rec := doJSON(t, h, http.MethodPost, "/v1/me/race-records", me, map[string]any{"raced_on": "2026-05-03", "event": "100m", "mark": 1100, "meet_name": "記録会", "round": round})
		if rec.Code != http.StatusCreated {
			t.Fatalf("create round=%q: %d %s", round, rec.Code, rec.Body.String())
		}
	}
	got := decodeRaceRecords(t, doJSON(t, h, http.MethodGet, "/v1/me/race-records", me, nil).Body.Bytes())
	want := []string{"", "予選", "準決勝", "決勝"}
	if len(got.Records) != len(want) {
		t.Fatalf("lost rounds: %+v", got)
	}
	for i, r := range got.Records {
		if r.Round != want[i] {
			t.Fatalf("order: %+v", got)
		}
	}
	rec := doJSON(t, h, http.MethodPatch, "/v1/me/race-records/"+got.Records[0].ID, me, map[string]any{"raced_on": "2026-05-03", "event": "100m", "mark": 1100, "round": "第2レース"})
	if rec.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	updated := decodeRaceRecords(t, rec.Body.Bytes())
	for _, r := range updated.Records {
		if r.ID == got.Records[0].ID && r.Round != "第2レース" {
			t.Fatalf("edit lost: %+v", r)
		}
	}
}
