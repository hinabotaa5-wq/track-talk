package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

// 継続プランの感覚メモ共有（#868 API）。共有の有無は exclusion 行の有無で表す。

type sharedNoteView struct {
	ID     string `json:"id"`
	Body   string `json:"body"`
	Day    string `json:"day"`
	Shared bool   `json:"shared"`
}

type sharedNotesView struct {
	Notes      []sharedNoteView `json:"notes"`
	ViewClosed bool             `json:"view_closed"`
}

func listSharedNotes(t *testing.T, h http.Handler, tok, id string) (int, sharedNotesView) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/sense-notes", tok, nil)
	var out sharedNotesView
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Notes == nil {
		out.Notes = []sharedNoteView{}
	}
	return rec.Code, out
}

func insertWindowNote(t *testing.T, author uuid.UUID, body, day string) string {
	t.Helper()
	var id string
	queryRowArgs(t, `INSERT INTO sense_notes (id, author_id, body, day) VALUES (gen_random_uuid(), $1, $2, $3::date) RETURNING id::text`,
		[]any{author, body, day}, &id)
	return id
}

func contractRequester(t *testing.T, id string) uuid.UUID {
	t.Helper()
	var rid string
	queryRowArgs(t, `SELECT requester_id::text FROM coaching_contracts WHERE id = $1::uuid`, []any{id}, &rid)
	out, err := uuid.Parse(rid)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func todayJST() string {
	return time.Now().In(jst.Zone).Format(time.DateOnly)
}

// ウィンドウ境界：開始日（JST）の1ヶ月前の同日のメモは含まれ、前日のメモは含まれない。
// 月末クランプ：starts_at が 2026-03-31（JST）の契約では window_start が 2026-02-28 になる。
func TestContractSenseNoteWindowBounds(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	requester := contractRequester(t, id)
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at = '2026-03-31T00:00:00+09', ends_at = now() + interval '30 days' WHERE id = $1`, id)

	before := insertWindowNote(t, requester, "2月27日の感覚", "2026-02-27")
	start := insertWindowNote(t, requester, "2月28日の感覚", "2026-02-28")
	after := insertWindowNote(t, requester, "3月3日の感覚", "2026-03-03")
	future := insertWindowNote(t, requester, "明日の感覚", time.Now().In(jst.Zone).AddDate(0, 0, 1).Format(time.DateOnly))

	code, got := listSharedNotes(t, h, tu, id)
	if code != http.StatusOK {
		t.Fatalf("requester list %d %+v", code, got)
	}
	seen := map[string]sharedNoteView{}
	for _, n := range got.Notes {
		seen[n.ID] = n
	}
	if _, ok := seen[before]; ok {
		t.Fatal("day before window_start is listed")
	}
	for _, want := range []string{start, after} {
		n, ok := seen[want]
		if !ok {
			t.Fatalf("window note %s is missing", want)
		}
		if !n.Shared {
			t.Fatalf("default shared is not true %+v", n)
		}
	}
	if _, ok := seen[future]; ok {
		t.Fatal("future note is listed")
	}
	if got.ViewClosed {
		t.Fatalf("active requester view %+v", got)
	}

	// 通常の月：開始日 2026-05-15 → window_start は 2026-04-15。
	execSQLArgs(t, `UPDATE coaching_contracts SET starts_at = '2026-05-15T00:00:00+09' WHERE id = $1`, id)
	edge := insertWindowNote(t, requester, "4月15日の感覚", "2026-04-15")
	edgeOut := insertWindowNote(t, requester, "4月14日の感覚", "2026-04-14")
	_, got = listSharedNotes(t, h, tu, id)
	seen = map[string]sharedNoteView{}
	for _, n := range got.Notes {
		seen[n.ID] = n
	}
	if _, ok := seen[edge]; !ok {
		t.Fatal("window_start note is missing")
	}
	if _, ok := seen[edgeOut]; ok {
		t.Fatal("day before window_start is listed")
	}
	_ = tc
}

// requester の GET（active）：ウィンドウ内の全メモが返り、初期状態では全件 shared:true。
// 契約中に新規作成したメモ（今日の分）も shared:true で両者の GET に現れる。
func TestContractSenseNoteDefaultShared(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	requester := contractRequester(t, id)
	yesterday := time.Now().In(jst.Zone).AddDate(0, 0, -1).Format(time.DateOnly)
	n1 := insertWindowNote(t, requester, "昨日の接地の感覚", yesterday)
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", tu, map[string]any{"body": "今日の腕振りの感覚"}); rec.Code != http.StatusCreated {
		t.Fatalf("create today %d %s", rec.Code, rec.Body.String())
	}

	code, got := listSharedNotes(t, h, tu, id)
	if code != http.StatusOK || len(got.Notes) != 2 || got.ViewClosed {
		t.Fatalf("requester %+v", got)
	}
	for _, n := range got.Notes {
		if !n.Shared {
			t.Fatalf("not shared %+v", n)
		}
	}

	code, coachGot := listSharedNotes(t, h, tc, id)
	if code != http.StatusOK || len(coachGot.Notes) != 2 || coachGot.ViewClosed {
		t.Fatalf("coach %+v", coachGot)
	}
	coachSeen := map[string]bool{}
	for _, n := range coachGot.Notes {
		coachSeen[n.ID] = true
		if !n.Shared {
			t.Fatalf("coach sees unshared %+v", n)
		}
	}
	if !coachSeen[n1] {
		t.Fatal("coach misses yesterday note")
	}
}

// チェック外し：shared:false を POST すると、そのメモが coach の GET から完全に消える。
// shared:true で戻すと再び見える。同じ操作の連打でも 200 で状態が壊れない。
func TestContractSenseNoteUnshareReshare(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	requester := contractRequester(t, id)
	yesterday := time.Now().In(jst.Zone).AddDate(0, 0, -1).Format(time.DateOnly)
	secret := insertWindowNote(t, requester, "見せたくない接地の感覚", yesterday)
	visible := insertWindowNote(t, requester, "見せてよい腕振り", todayJST())
	sharePath := "/v1/coaching/contracts/" + id + "/sense-notes/" + secret + "/share"

	unshare := func(want bool) {
		t.Helper()
		for i := 0; i < 2; i++ {
			rec := doJSON(t, h, http.MethodPost, sharePath, tu, map[string]any{"shared": want})
			var out struct {
				Shared bool `json:"shared"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &out)
			if rec.Code != http.StatusOK || out.Shared != want {
				t.Fatalf("share %v iter %d: %d %s", want, i, rec.Code, rec.Body.String())
			}
		}
	}

	unshare(false)
	_, coachGot := listSharedNotes(t, h, tc, id)
	if len(coachGot.Notes) != 1 || coachGot.Notes[0].ID != visible {
		t.Fatalf("coach still sees unshared %+v", coachGot)
	}
	for _, n := range coachGot.Notes {
		if n.Body == "見せたくない接地の感覚" || n.Day == yesterday && n.ID == secret {
			t.Fatalf("unshared body leaks %+v", n)
		}
	}
	code, reqGot := listSharedNotes(t, h, tu, id)
	if code != http.StatusOK || len(reqGot.Notes) != 2 {
		t.Fatalf("requester %+v", reqGot)
	}
	for _, n := range reqGot.Notes {
		if n.ID == secret && n.Shared {
			t.Fatalf("requester still shared %+v", n)
		}
	}

	unshare(true)
	_, coachGot = listSharedNotes(t, h, tc, id)
	if len(coachGot.Notes) != 2 {
		t.Fatalf("coach after reshare %+v", coachGot)
	}
}

// coach の GET の状態制限：applied・accepted では空、active で共有分だけ、
// ended では notes:[] ＋ view_closed:true。ends_at を過ぎた active でも本文を返さない。
func TestContractSenseNoteCoachStateLimits(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	requester := contractRequester(t, id)
	note := insertWindowNote(t, requester, "共有中の接地の感覚", todayJST())

	for _, status := range []string{"applied", "accepted"} {
		if status == "accepted" {
			execSQLArgs(t, `UPDATE coaching_contracts SET status = 'accepted', starts_at = NULL, ends_at = NULL, start_by = now() + interval '72 hours' WHERE id = $1`, id)
		} else {
			execSQLArgs(t, `UPDATE coaching_contracts SET status = 'applied', starts_at = NULL, ends_at = NULL WHERE id = $1`, id)
		}
		for _, tok := range []string{tu, tc} {
			code, got := listSharedNotes(t, h, tok, id)
			if code != http.StatusOK || len(got.Notes) != 0 || got.ViewClosed {
				t.Fatalf("%s %+v", status, got)
			}
		}
	}

	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'active', starts_at = now() - interval '10 days', ends_at = now() + interval '20 days' WHERE id = $1`, id)
	if code, got := listSharedNotes(t, h, tc, id); code != http.StatusOK || len(got.Notes) != 1 || got.Notes[0].ID != note {
		t.Fatalf("active coach %+v", got)
	}

	// 終わる時刻を過ぎたら、終わった印が付く前でもコーチには本文を返さない。選手の管理は開いたまま。
	execSQLArgs(t, `UPDATE coaching_contracts SET ends_at = now() - interval '1 minute' WHERE id = $1`, id)
	code, coachGot := listSharedNotes(t, h, tc, id)
	if code != http.StatusOK || len(coachGot.Notes) != 0 || !coachGot.ViewClosed ||
		strings.Contains(jsonString(t, coachGot), "共有中の接地の感覚") {
		t.Fatalf("coach after ends_at %+v", coachGot)
	}
	if code, reqGot := listSharedNotes(t, h, tu, id); code != http.StatusOK || len(reqGot.Notes) != 1 {
		t.Fatalf("requester after ends_at %+v", reqGot)
	}

	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'ended', ends_at = now() - interval '1 second' WHERE id = $1`, id)
	for _, tok := range []string{tu, tc} {
		code, got := listSharedNotes(t, h, tok, id)
		if code != http.StatusOK || len(got.Notes) != 0 || !got.ViewClosed {
			t.Fatalf("ended %+v", got)
		}
	}
}

func jsonString(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// 権限：当事者でない GET は 404、coach の POST は 404、他人のメモ・ウィンドウ外の自分のメモへの POST は 404、
// active でない契約への POST は 409。
func TestContractSenseNoteAuthMatrix(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	requester := contractRequester(t, id)
	mine := insertWindowNote(t, requester, "自分の今日の感覚", todayJST())
	old := insertWindowNote(t, requester, "3ヶ月前の感覚",
		time.Now().In(jst.Zone).AddDate(0, -3, 0).Format(time.DateOnly))
	otherID := uuid.New()
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", token(t, otherID), nil); rec.Code != http.StatusOK {
		t.Fatalf("other me %d", rec.Code)
	}
	other := insertWindowNote(t, otherID, "他人の感覚", todayJST())

	if code, _ := listSharedNotes(t, h, token(t, uuid.New()), id); code != http.StatusNotFound {
		t.Fatalf("outsider get %d", code)
	}
	post := func(tok, note string, body any) (int, string) {
		t.Helper()
		rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+id+"/sense-notes/"+note+"/share", tok, body)
		return rec.Code, rec.Body.String()
	}
	if code, _ := post(tc, mine, map[string]any{"shared": false}); code != http.StatusNotFound {
		t.Fatalf("coach post %d", code)
	}
	if code, _ := post(token(t, uuid.New()), mine, map[string]any{"shared": false}); code != http.StatusNotFound {
		t.Fatalf("outsider post %d", code)
	}
	if code, _ := post(tu, other, map[string]any{"shared": false}); code != http.StatusNotFound {
		t.Fatalf("other note %d", code)
	}
	if code, _ := post(tu, old, map[string]any{"shared": false}); code != http.StatusNotFound {
		t.Fatalf("out-of-window note %d", code)
	}
	if code, _ := post(tu, uuid.NewString(), map[string]any{"shared": false}); code != http.StatusNotFound {
		t.Fatalf("missing note %d", code)
	}
	if code, _ := post(tu, mine, map[string]any{}); code != http.StatusBadRequest {
		t.Fatalf("missing shared %d", code)
	}

	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'ended', ends_at = now() - interval '1 second' WHERE id = $1`, id)
	if code, body := post(tu, mine, map[string]any{"shared": false}); code != http.StatusConflict || !strings.Contains(body, "contract_not_active") {
		t.Fatalf("ended post %d %s", code, body)
	}
	execSQLArgs(t, `UPDATE coaching_contracts SET status = 'declined' WHERE id = $1`, id)
	if code, body := post(tu, mine, map[string]any{"shared": true}); code != http.StatusConflict || !strings.Contains(body, "contract_not_active") {
		t.Fatalf("declined post %d %s", code, body)
	}
}

// 既存の感覚メモ経路が不変：本人固定・1日1件・今日だけ。
func TestContractSenseNoteSelfRoutesUnchanged(t *testing.T) {
	h := setup(t)
	me := uuid.New()
	mine := token(t, me)
	other := token(t, uuid.New())

	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", mine, map[string]any{"body": "自分の接地の感覚"}); rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", mine, map[string]any{"body": "同じ日の2つ目"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("second %d", rec.Code)
	}
	var list struct {
		Notes []senseNoteItem `json:"notes"`
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", other, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Notes) != 0 {
		t.Fatalf("other sees %+v", list)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/sense-notes", mine, nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Notes) != 1 {
		t.Fatalf("mine %+v", list)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/sense-notes/"+list.Notes[0].ID, other, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other delete %d", rec.Code)
	}
}

// 保護者・管理の経路に感覚メモが漏れない。保護者用の新ルートは増えていない。
func TestContractSenseNoteGuardianLeak(t *testing.T) {
	h := setup(t)
	tm, gt := guardianMinor(t, h)
	tc, id := minorAccepted(t, h, tm)
	gpath := "/v1/guardian/" + gt + "/contracts/" + id
	if rec := doJSON(t, h, http.MethodPost, gpath+"/consent", "", map[string]any{"agree_report": true, "agree_detection": true}); rec.Code != http.StatusOK {
		t.Fatalf("guardian consent %d %s", rec.Code, rec.Body.String())
	}
	minor := contractRequester(t, id)
	const secret = "保護者に見せない接地の感覚XYZ"
	insertWindowNote(t, minor, secret, todayJST())

	rec := doJSON(t, h, http.MethodGet, gpath, "", nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("guardian contract %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, gpath+"/messages", "", nil)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("guardian messages %d %s", rec.Code, rec.Body.String())
	}
	// 保護者用の共有ルートは存在しない。
	rec = doJSON(t, h, http.MethodGet, gpath+"/sense-notes", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("guardian sense-notes %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, gpath+"/sense-notes/"+uuid.NewString()+"/share", "", map[string]any{"shared": false})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("guardian share %d", rec.Code)
	}
	// 通常の共有 GET 自体は選手とコーチに見える。
	if code, got := listSharedNotes(t, h, tm, id); code != http.StatusOK || len(got.Notes) != 1 {
		t.Fatalf("requester %+v", got)
	}
	if code, got := listSharedNotes(t, h, tc, id); code != http.StatusOK || len(got.Notes) != 1 {
		t.Fatalf("coach %+v", got)
	}
	_ = tc
}
