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

// 継続プランの感覚メモ共有：同じ日に書き直したとき、消す前のメモの除外を新しいメモへ引き継ぐ（#905）。

func createTodayNote(t *testing.T, h http.Handler, tok, body string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", tok, map[string]any{"body": body})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Note senseNoteItem `json:"note"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Note.ID == "" {
		t.Fatalf("create body %s", rec.Body.String())
	}
	return out.Note.ID
}

func deleteNote(t *testing.T, h http.Handler, tok, id string) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/sense-notes/"+id, tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
}

func setShared(t *testing.T, h http.Handler, tok, contract, note string, shared bool) {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/coaching/contracts/"+contract+"/sense-notes/"+note+"/share", tok, map[string]any{"shared": shared})
	if rec.Code != http.StatusOK {
		t.Fatalf("share %v %d %s", shared, rec.Code, rec.Body.String())
	}
}

// coachSees は、コーチの一覧の応答に、メモの ID・本文が含まれるかと、件数を返す。
func coachSees(t *testing.T, h http.Handler, tok, contract, note, body string) (bool, int) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+contract+"/sense-notes", tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("coach list %d %s", rec.Code, rec.Body.String())
	}
	raw := rec.Body.String()
	_, got := listSharedNotes(t, h, tok, contract)
	idIn := strings.Contains(raw, note)
	bodyIn := strings.Contains(raw, body)
	if idIn != bodyIn {
		t.Fatalf("id and body disagree id=%v body=%v %s", idIn, bodyIn, raw)
	}
	return idIn, len(got.Notes)
}

func requesterShared(t *testing.T, h http.Handler, tok, contract, note string) bool {
	t.Helper()
	_, got := listSharedNotes(t, h, tok, contract)
	for _, n := range got.Notes {
		if n.ID == note {
			return n.Shared
		}
	}
	t.Fatalf("requester misses note %s %+v", note, got)
	return false
}

func rewriteRecords(t *testing.T, author uuid.UUID) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM sense_note_rewrite_exclusions WHERE author_id = $1`, []any{author}, &n)
	return n
}

// 細目1：外したメモを消して同じ日に書き直すと、新しいメモはコーチの一覧に出ず（ID・本文・日付・件数）、選手側では shared:false。
func TestSenseNoteRewriteKeepsExclusion(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	first := createTodayNote(t, h, tu, "見せたくない最初の感覚")
	setShared(t, h, tu, id, first, false)
	deleteNote(t, h, tu, first)
	const secret = "書き直した見せたくない感覚"
	second := createTodayNote(t, h, tu, secret)

	seen, count := coachSees(t, h, tc, id, second, secret)
	if seen || count != 0 {
		t.Fatalf("coach sees rewritten note seen=%v count=%d", seen, count)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/coaching/contracts/"+id+"/sense-notes", tc, nil)
	if strings.Contains(rec.Body.String(), todayJST()) {
		t.Fatalf("coach sees today's day %s", rec.Body.String())
	}
	if requesterShared(t, h, tu, id, second) {
		t.Fatal("requester sees shared:true")
	}
}

// 細目2：外していないメモを消して書き直すと、新しいメモは通常どおり共有される。
func TestSenseNoteRewriteSharedStaysShared(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	first := createTodayNote(t, h, tu, "共有中の最初の感覚")
	deleteNote(t, h, tu, first)
	second := createTodayNote(t, h, tu, "書き直した共有の感覚")
	if seen, count := coachSees(t, h, tc, id, second, "書き直した共有の感覚"); !seen || count != 1 {
		t.Fatalf("coach misses rewritten note seen=%v count=%d", seen, count)
	}
	if !requesterShared(t, h, tu, id, second) {
		t.Fatal("requester sees shared:false")
	}
}

// 細目3：契約Aだけ外したメモを書き直すと、A のコーチには見えず、B のコーチには見える。
func TestSenseNoteRewritePerContract(t *testing.T) {
	h := setup(t)
	tcA, tu, idA := activeContract(t, h)
	tcB, _, idB := activeContract(t, h)
	// 2つ目の契約を同じ選手のものにする（コーチは別なので、開いている契約の重複にはならない）。
	execSQLArgs(t, `UPDATE coaching_contracts SET requester_id = $1 WHERE id = $2`, contractRequester(t, idA), idB)

	first := createTodayNote(t, h, tu, "Aにだけ見せない感覚")
	setShared(t, h, tu, idA, first, false)
	deleteNote(t, h, tu, first)
	second := createTodayNote(t, h, tu, "書き直したAにだけ見せない感覚")

	if seen, count := coachSees(t, h, tcA, idA, second, "書き直したAにだけ見せない感覚"); seen || count != 0 {
		t.Fatalf("coach A sees seen=%v count=%d", seen, count)
	}
	if seen, count := coachSees(t, h, tcB, idB, second, "書き直したAにだけ見せない感覚"); !seen || count != 1 {
		t.Fatalf("coach B misses seen=%v count=%d", seen, count)
	}
	if requesterShared(t, h, tu, idA, second) || !requesterShared(t, h, tu, idB, second) {
		t.Fatal("requester shared flags per contract are wrong")
	}
}

// 細目4：消す直前に共有を付け直したメモを書き直すと、引き継ぎは起きない。
func TestSenseNoteRewriteAfterReshare(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	first := createTodayNote(t, h, tu, "一度外して戻した感覚")
	setShared(t, h, tu, id, first, false)
	setShared(t, h, tu, id, first, true)
	deleteNote(t, h, tu, first)
	second := createTodayNote(t, h, tu, "戻したあと書き直した感覚")
	if seen, count := coachSees(t, h, tc, id, second, "戻したあと書き直した感覚"); !seen || count != 1 {
		t.Fatalf("coach misses seen=%v count=%d", seen, count)
	}

	// 一度外した状態で消して、書き直す前に（その日の）別の削除で状態が上書きされる形も確かめる：
	// 外さずに消した2回目の削除の状態が正になり、引き継がれない。
	setShared(t, h, tu, id, second, false)
	deleteNote(t, h, tu, second)
	third := createTodayNote(t, h, tu, "外して消した直後の感覚")
	setShared(t, h, tu, id, third, true)
	deleteNote(t, h, tu, third)
	fourth := createTodayNote(t, h, tu, "戻して消したあとの感覚")
	if seen, _ := coachSees(t, h, tc, id, fourth, "戻して消したあとの感覚"); !seen {
		t.Fatal("exclusion revived from an older delete")
	}
}

// 細目5：削除→書き直し→削除→書き直しでも、除外は引き継がれたまま。
func TestSenseNoteRewriteRepeated(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	note := createTodayNote(t, h, tu, "外した感覚0")
	setShared(t, h, tu, id, note, false)
	for i, body := range []string{"外した感覚1", "外した感覚2", "外した感覚3"} {
		deleteNote(t, h, tu, note)
		note = createTodayNote(t, h, tu, body)
		if seen, count := coachSees(t, h, tc, id, note, body); seen || count != 0 {
			t.Fatalf("round %d coach sees seen=%v count=%d", i, seen, count)
		}
		if requesterShared(t, h, tu, id, note) {
			t.Fatalf("round %d requester shared:true", i)
		}
	}
}

// 細目6：別の日のメモには引き継がない。前日に外したメモを消しても、今日のメモは既定で共有。
// 前日の日付の記録が残っていても、今日の作成には使われない。
func TestSenseNoteRewriteOtherDay(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	requester := contractRequester(t, id)
	yesterday := time.Now().In(jst.Zone).AddDate(0, 0, -1).Format(time.DateOnly)
	old := insertWindowNote(t, requester, "昨日の見せない感覚", yesterday)
	setShared(t, h, tu, id, old, false)
	deleteNote(t, h, tu, old)
	if n := rewriteRecords(t, requester); n != 0 {
		t.Fatalf("records kept for another day %d", n)
	}
	// 日付をまたいだ書き直しを模して、前日の記録を直接置く。
	execSQLArgs(t, `INSERT INTO sense_note_rewrite_exclusions (author_id, contract_id, day) VALUES ($1, $2::uuid, $3::date)`, requester, id, yesterday)

	today := createTodayNote(t, h, tu, "今日の共有する感覚")
	if seen, count := coachSees(t, h, tc, id, today, "今日の共有する感覚"); !seen || count != 1 {
		t.Fatalf("coach misses today's note seen=%v count=%d", seen, count)
	}
	if n := rewriteRecords(t, requester); n != 0 {
		t.Fatalf("records left after create %d", n)
	}
}

// 細目7：引き継がれた除外は、既存の共有操作で戻せる。
func TestSenseNoteRewriteInheritedCanReshare(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	first := createTodayNote(t, h, tu, "外した最初の感覚")
	setShared(t, h, tu, id, first, false)
	deleteNote(t, h, tu, first)
	second := createTodayNote(t, h, tu, "あとで見せる感覚")
	if seen, _ := coachSees(t, h, tc, id, second, "あとで見せる感覚"); seen {
		t.Fatal("coach sees inherited exclusion")
	}
	setShared(t, h, tu, id, second, true)
	if seen, count := coachSees(t, h, tc, id, second, "あとで見せる感覚"); !seen || count != 1 {
		t.Fatalf("reshare did not work seen=%v count=%d", seen, count)
	}
	if !requesterShared(t, h, tu, id, second) {
		t.Fatal("requester still shared:false")
	}
}

// 細目8：対象契約の無い選手は、作成・削除が従来どおり動き、記録も残らない。
// ほかの選手の記録はこの選手に影響しない。
func TestSenseNoteRewriteNoContract(t *testing.T) {
	h := setup(t)
	_, tu, id := activeContract(t, h)
	other := contractRequester(t, id)
	first := createTodayNote(t, h, tu, "契約のある選手の外した感覚")
	setShared(t, h, tu, id, first, false)
	deleteNote(t, h, tu, first)
	if n := rewriteRecords(t, other); n != 1 {
		t.Fatalf("records for requester %d", n)
	}

	me := uuid.New()
	tok := token(t, me)
	note := createTodayNote(t, h, tok, "契約の無い選手の感覚")
	deleteNote(t, h, tok, note)
	if n := rewriteRecords(t, me); n != 0 {
		t.Fatalf("records for no-contract user %d", n)
	}
	again := createTodayNote(t, h, tok, "契約の無い選手の書き直し")
	var excluded int
	queryRowArgs(t, `SELECT count(*)::int FROM sense_note_contract_exclusions WHERE sense_note_id = $1::uuid`, []any{again}, &excluded)
	if excluded != 0 {
		t.Fatalf("exclusions for no-contract user %d", excluded)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", tok, map[string]any{"body": "同じ日の2つ目"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("second %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/sense-notes/"+uuid.NewString(), tok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing delete %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me/sense-notes/"+again, token(t, uuid.New()), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("other delete %d", rec.Code)
	}
	// 契約のある選手の記録は、この選手の操作で消えない。
	if n := rewriteRecords(t, other); n != 1 {
		t.Fatalf("other's records changed %d", n)
	}
}
