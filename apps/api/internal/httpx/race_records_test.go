package httpx_test

import (
	"encoding/json"
	"net/http"
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

	rec := doJSON(t, h, http.MethodPost, "/v1/me/race-records", me, map[string]any{
		"raced_on": "2026-05-03", "event": "100m", "mark": 1112, "wind": 8, "memo": "スタートで出遅れた",
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
	if got.Records[0].Memo != "スタートで出遅れた" || got.Records[0].Wind == nil || *got.Records[0].Wind != 8 {
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
	if got := decodeRaceRecords(t, rec.Body.Bytes()); got.Records[0].Mark != 1102 || got.Records[0].Memo != "" {
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
