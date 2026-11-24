package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5"
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

// 共有の切り替えと削除の直列化（#1001）。削除の取引を「退避の直後・削除の前」で止めたまま共有を切り替え、
// 切り替えが作者ロックを待つこと、削除の後は 404 になること、保存された状態が最後に確定した操作と一致することを確かめる。
// 待ちの判定は時間ではなく、DB 上で作者ロック（advisory）を待っている接続があるかで行う。

// pausedDelete は、削除の取引（作者ロック→メモの確認→退避）を進めたところで止めたもの。commit で削除して確定する。
type pausedDelete struct {
	t      *testing.T
	tx     pgx.Tx
	q      *db.Queries
	note   uuid.UUID
	author uuid.UUID
}

func pauseDeleteAfterSave(t *testing.T, author uuid.UUID, note string) *pausedDelete {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	q := db.New(tx)
	noteID := uuid.MustParse(note)
	if err := q.LockSenseNoteAuthor(ctx, author); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetContractSenseNote(ctx, db.GetContractSenseNoteParams{ID: noteID, AuthorID: author})
	if err != nil {
		t.Fatal(err)
	}
	y, m, d := time.Now().In(jst.Zone).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	ny, nm, nd := got.Day.Date()
	if err := q.ClearSenseNoteRewriteExclusions(ctx, db.ClearSenseNoteRewriteExclusionsParams{
		AuthorID: author, NoteDay: time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC), Today: today,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.SaveSenseNoteRewriteExclusions(ctx, db.SaveSenseNoteRewriteExclusionsParams{
		NoteID: noteID, AuthorID: author, Today: today,
	}); err != nil {
		t.Fatal(err)
	}
	return &pausedDelete{t: t, tx: tx, q: q, note: noteID, author: author}
}

func (p *pausedDelete) commit() {
	p.t.Helper()
	ctx := context.Background()
	if n, err := p.q.DeleteSenseNote(ctx, db.DeleteSenseNoteParams{ID: p.note, AuthorID: p.author}); err != nil || n != 1 {
		p.t.Fatalf("paused delete n=%d err=%v", n, err)
	}
	if err := p.tx.Commit(ctx); err != nil {
		p.t.Fatal(err)
	}
}

// holdAuthorLock は、作者ロックだけを取って止めた取引。release で放す。
func holdAuthorLock(t *testing.T, author uuid.UUID) (release func()) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.New(tx).LockSenseNoteAuthor(ctx, author); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

type asyncResult struct {
	code int
	body string
}

// asyncJSON は要求を別の goroutine で送り、結果をチャネルで返す（goroutine の中では t.Fatal しない）。
func asyncJSON(t *testing.T, h http.Handler, method, path, tok string, body any) <-chan asyncResult {
	t.Helper()
	ch := make(chan asyncResult, 1)
	go func() {
		rec := doJSON(t, h, method, path, tok, body)
		ch <- asyncResult{code: rec.Code, body: rec.Body.String()}
	}()
	return ch
}

// advisoryWaiters は、advisory ロックを待っている接続の数を返す。
func advisoryWaiters(t *testing.T) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock' AND wait_event = 'advisory'`, nil, &n)
	return n
}

// waitAdvisoryWaiters は、advisory ロックの待ちが want 件になるまで確かめる。ch がその前に返ってきたら、
// 待たずに通った（直列になっていない）として失敗にする。上限は異常時に止めるためだけのもの。
func waitAdvisoryWaiters(t *testing.T, want int, chs ...<-chan asyncResult) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for advisoryWaiters(t) < want {
		for _, ch := range chs {
			select {
			case got := <-ch:
				t.Fatalf("request finished without waiting for the author lock: %d %s", got.code, got.body)
			default:
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no request is waiting for the author lock (want %d)", want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func receive(t *testing.T, ch <-chan asyncResult) asyncResult {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(10 * time.Second):
		t.Fatal("request did not finish")
	}
	return asyncResult{}
}

func exclusionRows(t *testing.T, contract, note string) int {
	t.Helper()
	var n int
	queryRowArgs(t, `SELECT count(*)::int FROM sense_note_contract_exclusions WHERE contract_id = $1::uuid AND sense_note_id = $2::uuid`,
		[]any{contract, note}, &n)
	return n
}

func sharePath(contract, note string) string {
	return "/v1/coaching/contracts/" + contract + "/sense-notes/" + note + "/share"
}

// T1：削除の退避と削除のあいだに「外す」を送ると、切り替えは作者ロックを待ち、削除の確定後に 404 になる。
// 確定した最後の操作は削除（その時点の共有）なので、同じ日に書き直したメモは共有される。除外の行は残らない。
func TestSenseNoteShareWaitsForPausedDelete(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	author := contractRequester(t, id)
	first := createTodayNote(t, h, tu, "消す途中の感覚")

	paused := pauseDeleteAfterSave(t, author, first)
	ch := asyncJSON(t, h, http.MethodPost, sharePath(id, first), tu, map[string]any{"shared": false})
	waitAdvisoryWaiters(t, 1, ch)
	paused.commit()
	got := receive(t, ch)
	if got.code != http.StatusNotFound || !strings.Contains(got.body, "見つかりませんでした") {
		t.Fatalf("unshare after delete %d %s", got.code, got.body)
	}
	if n := exclusionRows(t, id, first); n != 0 {
		t.Fatalf("exclusion rows for deleted note %d", n)
	}
	if n := rewriteRecords(t, author); n != 0 {
		t.Fatalf("saved records %d", n)
	}
	const body = "書き直した感覚（共有のまま）"
	second := createTodayNote(t, h, tu, body)
	if seen, _ := coachSees(t, h, tc, id, second, body); !seen {
		t.Fatal("coach misses rewritten note that was shared when deleted")
	}
	if !requesterShared(t, h, tu, id, second) {
		t.Fatal("requester sees shared:false")
	}
}

// T1（逆の順）：「外す」が先に作者ロックを取ると、削除はその後に除外を退避し、書き直したメモはコーチに見えない。
func TestSenseNoteUnshareBeforeDeleteKeepsExclusion(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	author := contractRequester(t, id)
	first := createTodayNote(t, h, tu, "外してから消す感覚")

	release := holdAuthorLock(t, author)
	share := asyncJSON(t, h, http.MethodPost, sharePath(id, first), tu, map[string]any{"shared": false})
	waitAdvisoryWaiters(t, 1, share)
	del := asyncJSON(t, h, http.MethodDelete, "/v1/me/sense-notes/"+first, tu, nil)
	waitAdvisoryWaiters(t, 2, share, del)
	release()
	if got := receive(t, share); got.code != http.StatusOK || !strings.Contains(got.body, `"shared":false`) {
		t.Fatalf("unshare %d %s", got.code, got.body)
	}
	if got := receive(t, del); got.code != http.StatusOK {
		t.Fatalf("delete %d %s", got.code, got.body)
	}
	if n := rewriteRecords(t, author); n != 1 {
		t.Fatalf("saved records %d", n)
	}
	const secret = "書き直した見せたくない感覚"
	second := createTodayNote(t, h, tu, secret)
	if seen, count := coachSees(t, h, tc, id, second, secret); seen || count != 0 {
		t.Fatalf("coach sees rewritten note seen=%v count=%d", seen, count)
	}
	if requesterShared(t, h, tu, id, second) {
		t.Fatal("requester sees shared:true")
	}
}

// T2：外したメモの削除を退避の直後で止め、その間に「戻す」を送ると、切り替えは待ってから 404 になる。
// 退避した除外は残り、書き直したメモはコーチに見えない（確定した最後の操作は「外す」）。
func TestSenseNoteReshareWaitsForPausedDelete(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	author := contractRequester(t, id)
	first := createTodayNote(t, h, tu, "外して消す途中の感覚")
	setShared(t, h, tu, id, first, false)

	paused := pauseDeleteAfterSave(t, author, first)
	ch := asyncJSON(t, h, http.MethodPost, sharePath(id, first), tu, map[string]any{"shared": true})
	waitAdvisoryWaiters(t, 1, ch)
	paused.commit()
	if got := receive(t, ch); got.code != http.StatusNotFound || !strings.Contains(got.body, "見つかりませんでした") {
		t.Fatalf("reshare after delete %d %s", got.code, got.body)
	}
	if n := rewriteRecords(t, author); n != 1 {
		t.Fatalf("saved records %d", n)
	}
	const secret = "書き直した外したままの感覚"
	second := createTodayNote(t, h, tu, secret)
	if seen, count := coachSees(t, h, tc, id, second, secret); seen || count != 0 {
		t.Fatalf("coach sees rewritten note seen=%v count=%d", seen, count)
	}
	if requesterShared(t, h, tu, id, second) {
		t.Fatal("requester sees shared:true")
	}
}

// T2（逆の順）：「戻す」が先に作者ロックを取ると、削除の時点では共有中なので、書き直したメモは共有される。
func TestSenseNoteReshareBeforeDeleteShares(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	author := contractRequester(t, id)
	first := createTodayNote(t, h, tu, "戻してから消す感覚")
	setShared(t, h, tu, id, first, false)

	release := holdAuthorLock(t, author)
	share := asyncJSON(t, h, http.MethodPost, sharePath(id, first), tu, map[string]any{"shared": true})
	waitAdvisoryWaiters(t, 1, share)
	del := asyncJSON(t, h, http.MethodDelete, "/v1/me/sense-notes/"+first, tu, nil)
	waitAdvisoryWaiters(t, 2, share, del)
	release()
	if got := receive(t, share); got.code != http.StatusOK || !strings.Contains(got.body, `"shared":true`) {
		t.Fatalf("reshare %d %s", got.code, got.body)
	}
	if got := receive(t, del); got.code != http.StatusOK {
		t.Fatalf("delete %d %s", got.code, got.body)
	}
	if n := rewriteRecords(t, author); n != 0 {
		t.Fatalf("saved records %d", n)
	}
	const body = "書き直した戻した感覚"
	second := createTodayNote(t, h, tu, body)
	if seen, _ := coachSees(t, h, tc, id, second, body); !seen {
		t.Fatal("coach misses rewritten note")
	}
}

// T3：外す→消す→同じ日に書き直す、を競合なしで順に行うと、書き直したメモはコーチに見えない。
// 削除の途中で「外す」が来た T1 の経路でも、確定前の「外す」が後から除外を作って意図せず状態を変えることはない
// （T1 で 404・除外0件を確認）。ここでは、切り替えを取引にしたあとも順の操作の結果が変わらないことを確かめる。
func TestSenseNoteUnshareDeleteRewriteSequential(t *testing.T) {
	h := setup(t)
	tc, tu, id := activeContract(t, h)
	first := createTodayNote(t, h, tu, "順に外して消す感覚")
	setShared(t, h, tu, id, first, false)
	deleteNote(t, h, tu, first)
	const secret = "順に書き直した見せたくない感覚"
	second := createTodayNote(t, h, tu, secret)
	if seen, count := coachSees(t, h, tc, id, second, secret); seen || count != 0 {
		t.Fatalf("coach sees rewritten note seen=%v count=%d", seen, count)
	}
	if exclusionRows(t, id, second) != 1 {
		t.Fatal("rewritten note lacks exclusion")
	}
}
