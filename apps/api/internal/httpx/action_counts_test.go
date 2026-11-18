package httpx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/migrate"
	"github.com/jackc/pgx/v5"
)

// Stored coaching notices still exist, but none of their IDs may appear in the bell.
// Existing delivery/push tests use this instead of treating the bell as a list of every notice.
func coachingNotifications(t *testing.T, h http.Handler, tok string) []noteItem {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	rows, err := conn.Query(context.Background(), `SELECT id::text, kind, COALESCE(title,''), COALESCE(body,''), COALESCE(url,''), read_at IS NOT NULL
		FROM notifications WHERE recipient_id=$1 AND coaching_role IS NOT NULL`, tokenSubject(t, tok))
	if err != nil {
		t.Fatal(err)
	}
	var notices []noteItem
	for rows.Next() {
		var n noteItem
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.URL, &n.Read); err != nil {
			t.Fatal(err)
		}
		notices = append(notices, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	bell, _ := listNotifications(t, h, tok)
	for _, n := range notices {
		for _, b := range bell {
			if b.ID == n.ID {
				t.Fatalf("coaching notice %s leaked into bell", n.ID)
			}
		}
	}
	return notices
}

func circleNotice(t *testing.T, to uuid.UUID, kind, role, url string, request *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	actionSeed(t, `INSERT INTO notifications(id, recipient_id, kind, coaching_role, url, review_request_id, title, body, post_id)
		VALUES($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,'notice','',CASE WHEN $6::uuid IS NULL AND $3 <> 'message' THEN gen_random_uuid() END)`, id, to, kind, role, url, request)
	return id
}

func circleReview(t *testing.T, requester, coach uuid.UUID, status string) uuid.UUID {
	t.Helper()
	seedActionReview(t, requester, coach, status)
	var id uuid.UUID
	queryRowArgs(t, `SELECT id FROM review_requests WHERE requester_id=$1 AND coach_id=$2 AND status=$3 ORDER BY created_at DESC LIMIT 1`, []any{requester, coach, status}, &id)
	return id
}

func circleGet(t *testing.T, h http.Handler, tok, path string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, path, tok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
	}
}

func circleUnread(t *testing.T, id uuid.UUID, want bool) {
	t.Helper()
	var unread bool
	queryRowArgs(t, `SELECT read_at IS NULL FROM notifications WHERE id=$1`, []any{id}, &unread)
	if unread != want {
		t.Fatalf("notice %s unread=%v want %v", id, unread, want)
	}
}

// A1-A3, B5/B10, C11/C12: the bell never clears coaching rows; opening an owned detail does.
func TestCircleReturnedReviewBellAndReadIsolation(t *testing.T) {
	h := setup(t)
	player, coach := uuid.New(), uuid.New()
	tu, outsider := token(t, player), token(t, uuid.New())
	id := circleReview(t, player, coach, "returned")
	n := circleNotice(t, player, "review_returned", "player", "", &id)
	otherID := circleReview(t, player, coach, "returned")
	other := circleNotice(t, player, "review_returned", "player", "/coaching/requests/"+otherID.String(), nil)
	bell := circleNotice(t, player, "message", "", "/admin", nil)
	items, unread := listNotifications(t, h, tu)
	if len(items) != 1 || unread != 1 || items[0].ID != bell.String() {
		t.Fatalf("bell %+v unread=%d", items, unread)
	}
	wantActionCounts(t, h, tu, 2, 0)
	wantActionCounts(t, h, outsider, 0, 0)
	if r := doJSON(t, h, "POST", "/v1/me/notifications/read", tu, nil); r.Code != 204 {
		t.Fatal(r.Body.String())
	}
	circleUnread(t, n, true)
	circleUnread(t, other, true)
	circleUnread(t, bell, false)
	wantActionCounts(t, h, tu, 2, 0)
	if r := doJSON(t, h, "GET", "/v1/coaching/requests/"+id.String(), outsider, nil); r.Code != 404 {
		t.Fatal(r.Code)
	}
	circleUnread(t, n, true)
	// An administrator cannot mark a player's notice by opening the administration screen.
	_ = doJSON(t, h, "GET", "/v1/admin/coaching/requests/"+id.String(), adminToken(t, h), nil)
	circleUnread(t, n, true)
	circleGet(t, h, tu, "/v1/coaching/requests/"+id.String())
	circleUnread(t, n, false)
	circleUnread(t, other, true)
	wantActionCounts(t, h, tu, 1, 0)
}

// B6-B7/B9: actions retain their original meaning and suppress only their matching unread notices.
func TestCircleActionNotificationDeduplication(t *testing.T) {
	h := setup(t)
	player, coach := uuid.New(), uuid.New()
	tu, tc := token(t, player), token(t, coach)
	id := circleReview(t, player, coach, "queued")
	n := circleNotice(t, coach, "review_offer", "coach", "", &id)
	wantActionCounts(t, h, tc, 0, 1)
	actionSeed(t, `UPDATE review_requests SET status='in_review' WHERE id=$1`, id)
	wantActionCounts(t, h, tc, 0, 1)
	circleUnread(t, n, true)
	info := circleReview(t, player, coach, "needs_info")
	circleNotice(t, player, "review_info_request", "player", "", &info)
	wantActionCounts(t, h, tu, 1, 0)
	actionSeed(t, `UPDATE review_requests SET status='queued', info_request=NULL, info_requested_at=NULL, info_from_status=NULL, resubmit_by=NULL WHERE id=$1`, info)
	// Re-submission's response builder is a mutation, so it does not itself mark notices read.
	// Explicitly finish reading before checking that changing the action removed the remaining count.
	circleGet(t, h, tu, "/v1/coaching/requests/"+info.String())
	wantActionCounts(t, h, tu, 0, 0)
	seedActionContract(t, player, coach, "applied")
	seedActionSupport(t, player, coach, "requested")
	circleNotice(t, coach, "message", "coach", "/coach/contracts", nil)
	circleNotice(t, coach, "message", "coach", "/coach/support-consults", nil)
	wantActionCounts(t, h, tc, 0, 3)
	// Work-order links have a fragment but refer to the same dependency already counted in (a).
	circleNotice(t, coach, "message", "coach", "/coach/requests/"+id.String()+"#work-order", nil)
	wantActionCounts(t, h, tc, 0, 3)
}

// B8/C11: active contract chat counts once per recipient/role and clears only for the viewer.
func TestCircleContractAndScreenReads(t *testing.T) {
	h := setup(t)
	coach, tc := approvedCoachWithServices(t, h, nil)
	// Start from an acknowledged application result, before isolating contract chat notices.
	circleGet(t, h, tc, "/v1/coach/me")
	player := uuid.New()
	tu := token(t, player)
	circleGet(t, h, tu, "/v1/me")
	seedActionContract(t, player, coach, "active")
	var id uuid.UUID
	queryRowArgs(t, `SELECT id FROM coaching_contracts WHERE requester_id=$1`, []any{player}, &id)
	url := "/coaching/personal/" + id.String()
	pn := circleNotice(t, player, "message", "player", url, nil)
	cn := circleNotice(t, coach, "message", "coach", url, nil)
	wantActionCounts(t, h, tu, 1, 0)
	wantActionCounts(t, h, tc, 0, 1)
	circleGet(t, h, tu, "/v1/coaching/contracts/"+id.String())
	circleUnread(t, pn, false)
	circleUnread(t, cn, true)
	circleGet(t, h, tc, "/v1/coaching/contracts/"+id.String())
	wantActionCounts(t, h, tc, 0, 0)
	actionSeed(t, `UPDATE coaching_contracts SET status='accepted', starts_at=NULL, ends_at=NULL, start_by=now()+interval '1 day' WHERE id=$1`, id)
	circleNotice(t, player, "message", "player", url, nil)
	wantActionCounts(t, h, tu, 1, 0)
	// Dashboard and ticket screens clear only their own exact destinations and role.
	for _, screen := range []struct {
		tok             string
		to              uuid.UUID
		role, url, path string
	}{
		{tu, player, "player", "/me/coaching", "/v1/me/coaching"},
		{tu, player, "player", "/me/coaching/tickets", "/v1/me/coaching/tickets"},
		{tc, coach, "coach", "/coach", "/v1/coach/me"},
		{tc, coach, "coach", "/coach/apply", "/v1/coach/me"},
	} {
		t.Run(screen.url, func(t *testing.T) {
			n := circleNotice(t, screen.to, "message", screen.role, screen.url, nil)
			unrelated := circleNotice(t, screen.to, "message", screen.role, "/unrelated", nil)
			ordinary := circleNotice(t, screen.to, "message", "", "/admin", nil)
			circleGet(t, h, screen.tok, screen.path)
			circleUnread(t, n, false)
			circleUnread(t, unrelated, true)
			circleUnread(t, ordinary, true)
		})
	}
}

// C11: UPDATE failure is best effort and does not turn a successfully returned GET into an error.
func TestCircleReadFailureDoesNotFailGET(t *testing.T) {
	h := setup(t)
	player := uuid.New()
	tu := token(t, player)
	n := circleNotice(t, player, "message", "player", "/me/coaching", nil)
	remove := injectNotificationFailure(t, "notifications", "UPDATE")
	circleGet(t, h, tu, "/v1/me/coaching")
	circleUnread(t, n, true)
	remove()
	circleGet(t, h, tu, "/v1/me/coaching")
	circleUnread(t, n, false)
}

// E15/A4: apply the actual migration to pre-existing rows, including misleading announcement URLs.
// DDL is contained in a transaction and rolled back, so the shared migrated test schema is preserved.
func TestCircleMigrationBackfillAndDown(t *testing.T) {
	h := setup(t)
	_ = h
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`SET LOCAL session_replication_role=replica`)
	exec(`ALTER TABLE notifications DROP COLUMN coaching_role`)
	player, coach := uuid.New(), uuid.New()
	// Contract participation determines the role even when IDs and URL text were all that old rows held.
	id := uuid.New()
	exec(`INSERT INTO coaching_contracts(id,plan_id,coach_id,requester_id,plan_kind,plan_title,duration_days,price_yen,event,goal,current_record,message,minor,respond_by,status)
		VALUES($1,gen_random_uuid(),$2,$3,'period','plan',30,10000,'100m','goal','12','hello',false,now()+interval '1 day','applied')`, id, coach, player)
	for _, tc := range []struct {
		kind, url, title, role string
		to                     uuid.UUID
		announcement           bool
	}{
		{"review_returned", "", "", "player", player, false},
		{"review_offer", "", "", "coach", coach, false},
		{"review_offer_reminder", "", "", "coach", coach, false},
		{"review_resubmitted", "", "", "coach", coach, false},
		{"review_resubmitted_after_accept", "", "", "coach", coach, false},
		{"review_nomination_ended", "", "", "coach", coach, false},
		{"review_clarify", "", "", "coach", coach, false},
		{"message", "/coach", "", "coach", coach, false},
		{"message", "/coach/apply", "", "coach", coach, false},
		{"message", "/coaching/requests/" + uuid.NewString(), "", "player", player, false},
		{"message", "/coaching/support-consults/" + uuid.NewString(), "", "player", player, false},
		{"message", "/me/coaching", "", "player", player, false},
		{"message", "/me/coaching/tickets", "", "player", player, false},
		{"message", "/me", "紹介特典：test", "player", player, false},
		{"message", "/coaching/personal/" + id.String(), "", "player", player, false},
		{"message", "/coaching/personal/" + id.String(), "", "coach", coach, false},
		{"message", "/coaching/personal/" + uuid.NewString(), "", "", player, false},
		{"message", "/coaching/personal/not-a-uuid", "", "", player, false},
		{"message", "/admin", "", "", player, false},
		{"message", "/me", "monthly", "", player, false},
		{"message", "/coach/contracts", "announcement", "", coach, true},
		{"review_offer", "/coach", "announcement", "", coach, true},
	} {
		n := uuid.New()
		var ann *uuid.UUID
		if tc.announcement {
			a := uuid.New()
			ann = &a
		}
		exec(`INSERT INTO notifications(id,recipient_id,kind,url,title,announcement_id,body,review_request_id) VALUES($1,$2,$3,$4,$5,$6,'',CASE WHEN $3 <> 'message' THEN gen_random_uuid() END)`, n, tc.to, tc.kind, tc.url, tc.title, ann)
		// Remember the expected role independently of the migration's SQL.
		exec(`CREATE TEMP TABLE IF NOT EXISTS expected_roles(id uuid PRIMARY KEY, role text) ON COMMIT DROP`)
		exec(`INSERT INTO expected_roles VALUES($1,$2)`, n, tc.role)
	}
	up, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000185_notifications_coaching_role.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	exec(string(up))
	var bad int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM notifications n JOIN expected_roles e USING(id) WHERE coalesce(n.coaching_role,'')<>e.role`).Scan(&bad); err != nil || bad != 0 {
		t.Fatalf("backfill mismatches=%d err=%v", bad, err)
	}
	q := db.New(tx)
	bell, err := q.CountMyNotifications(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if bell != 4 {
		t.Fatalf("player bell after backfill=%d want 4", bell)
	}
	before, err := q.CountUnreadNotifications(ctx, coach)
	if err != nil {
		t.Fatal(err)
	}
	if before != 2 {
		t.Fatalf("announcements should stay in bell: %d", before)
	}
	down, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000185_notifications_coaching_role.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	exec(string(down))
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='notifications' AND column_name='coaching_role')`).Scan(&exists); err != nil || exists {
		t.Fatalf("down: exists=%v err=%v", exists, err)
	}
	var all int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE recipient_id=$1`, player).Scan(&all); err != nil || all <= int(bell) {
		t.Fatalf("down restores all bell rows=%d err=%v", all, err)
	}
	counts, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "cloud-run", "payment-counts", "counts.py"))
	if err != nil {
		t.Fatal(err)
	}
	latest, err := migrate.ExpectedVersion()
	if err != nil || !strings.Contains(string(counts), fmt.Sprintf("EXPECTED_MIGRATION = %d", latest)) {
		t.Fatalf("migration count does not match latest %d: %v", latest, err)
	}
}

// コーチタブの円ボタンのバッジ用の件数（本体 #891）。GET /v1/me/coaching/action-counts。
// 他テーブルへの外部キーは使わず件数の条件だけを確かめたいので、行は外部キー検査を外した接続で直接入れる。

type actionCountsView struct {
	PlayerCount *int64 `json:"player_count"`
	CoachCount  *int64 `json:"coach_count"`
}

func actionSeed(t *testing.T, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "SET session_replication_role = replica"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// 添削依頼を1件入れる。requester・coach の相手側は別人のランダムな ID。
func seedActionReview(t *testing.T, requester, coach uuid.UUID, status string) {
	t.Helper()
	actionSeed(t, `INSERT INTO review_requests (id, requester_id, coach_id, tier, event, focus, question, due_at, status,
			info_request, info_requested_at, info_from_status, resubmit_by)
		VALUES (gen_random_uuid(), $1, $2, 'coach', '100m', '{}', 'スタートを見てください', now() + interval '1 day', $3,
			CASE WHEN $3 = 'needs_info' THEN '質問' END,
			CASE WHEN $3 = 'needs_info' THEN now() END,
			CASE WHEN $3 = 'needs_info' THEN 'in_review' END,
			CASE WHEN $3 = 'needs_info' THEN now() + interval '1 day' END)`, requester, coach, status)
}

func seedActionSupport(t *testing.T, athlete, coach uuid.UUID, status string) {
	t.Helper()
	actionSeed(t, `INSERT INTO support_consults (id, athlete_id, coach_id, minor, event, answers, status, answer_by)
		VALUES (gen_random_uuid(), $1, $2, false, '100m', '{}'::jsonb, $3, now() + interval '1 day')`, athlete, coach, status)
}

func seedActionContract(t *testing.T, requester, coach uuid.UUID, status string) {
	t.Helper()
	actionSeed(t, `INSERT INTO coaching_contracts (id, plan_id, coach_id, requester_id, plan_kind, plan_title, duration_days, price_yen,
			event, goal, current_record, message, minor, respond_by, status, start_by, starts_at, ends_at)
		VALUES (gen_random_uuid(), gen_random_uuid(), $2, $1, 'period', 'プラン', 30, 10000,
			'100m', '目標', '12.0', 'よろしくお願いします', false, now() + interval '1 day', $3,
			CASE WHEN $3 = 'accepted' THEN now() + interval '2 days' END,
			CASE WHEN $3 IN ('active', 'ended') THEN now() END,
			CASE WHEN $3 IN ('active', 'ended') THEN now() + interval '30 days' END)`, requester, coach, status)
}

// 追加相談。clarify は answered のときの補足依頼の状態（none／asked／done）。
func seedActionConsultation(t *testing.T, requester, coach uuid.UUID, status, clarify string, purged bool) {
	t.Helper()
	actionSeed(t, `INSERT INTO coaching_consultations (id, request_key, review_request_id, requester_id, coach_id, minor, funding, price_yen, share_percent,
			status, answered_at, clarify_requested_at, clarified_at, purged_at)
		VALUES (gen_random_uuid(), gen_random_uuid(), gen_random_uuid(), $1, $2, false, 'cash', 500, 70,
			$3, CASE WHEN $3 = 'answered' THEN now() END,
			CASE WHEN $4 IN ('asked', 'done') THEN now() END,
			CASE WHEN $4 = 'done' THEN now() END,
			CASE WHEN $5 THEN now() END)`, requester, coach, status, clarify, purged)
}

func getActionCounts(t *testing.T, h http.Handler, tok string) (int, actionCountsView, map[string]json.RawMessage) {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching/action-counts", tok, nil)
	var out actionCountsView
	var raw map[string]json.RawMessage
	if rec.Code == http.StatusOK {
		dec := json.NewDecoder(strings.NewReader(rec.Body.String()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&out); err != nil {
			t.Fatalf("decode %v %s", err, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
	}
	return rec.Code, out, raw
}

func wantActionCounts(t *testing.T, h http.Handler, tok string, player, coach int64) {
	t.Helper()
	code, got, _ := getActionCounts(t, h, tok)
	if code != http.StatusOK || got.PlayerCount == nil || got.CoachCount == nil {
		t.Fatalf("action-counts %d %+v", code, got)
	}
	if *got.PlayerCount != player || *got.CoachCount != coach {
		t.Fatalf("player=%d coach=%d, want player=%d coach=%d", *got.PlayerCount, *got.CoachCount, player, coach)
	}
}

// 細目1: 選手分。数える4種と、数えない状態。
func TestActionCountsPlayer(t *testing.T) {
	h := setupConfig(t, nil)
	me := uuid.New()
	tok := token(t, me)
	other := uuid.New()

	for _, st := range []string{"needs_info", "awaiting_payment"} {
		seedActionReview(t, me, other, st)
	}
	seedActionSupport(t, me, uuid.New(), "proposed")
	seedActionContract(t, me, uuid.New(), "accepted")
	wantActionCounts(t, h, tok, 4, 0)

	// 数えない状態を足しても増えない。
	for _, st := range []string{"in_review", "returned", "expired", "queued", "offered"} {
		seedActionReview(t, me, other, st)
	}
	for _, st := range []string{"requested", "clarifying", "sent_to_guardian", "accepted", "declined"} {
		seedActionSupport(t, me, uuid.New(), st)
	}
	for _, st := range []string{"applied", "active", "ended"} {
		seedActionContract(t, me, uuid.New(), st)
	}
	wantActionCounts(t, h, tok, 4, 0)
}

// 細目2: コーチ分。
func TestActionCountsCoach(t *testing.T) {
	h := setupConfig(t, nil)
	me := uuid.New()
	tok := token(t, me)
	athlete := uuid.New()

	seedActionReview(t, athlete, me, "in_review")
	seedActionReview(t, athlete, me, "needs_info")
	seedActionConsultation(t, athlete, me, "offered", "none", false)
	seedActionConsultation(t, athlete, me, "in_review", "none", false)
	seedActionConsultation(t, athlete, me, "answered", "asked", false)
	seedActionContract(t, uuid.New(), me, "applied")
	// 無料相談は requested だけを数える（#891 補足 2026-11-17）。選手が回答して requested に戻った行もこの状態で数える。
	seedActionSupport(t, uuid.New(), me, "requested")
	wantActionCounts(t, h, tok, 0, 7)

	// 数えないもの。
	seedActionConsultation(t, athlete, me, "answered", "none", false) // 補足依頼なし
	seedActionConsultation(t, athlete, me, "answered", "done", false) // 補足に返事済み
	seedActionConsultation(t, athlete, me, "offered", "none", true)   // 消去済み
	seedActionConsultation(t, athlete, me, "declined", "none", false)
	seedActionSupport(t, uuid.New(), me, "proposed")
	// clarifying はコーチが確認質問を出して選手の回答待ちなので、コーチ分に数えない。
	seedActionSupport(t, uuid.New(), me, "clarifying")
	seedActionReview(t, athlete, me, "returned")
	seedActionContract(t, uuid.New(), me, "active")
	wantActionCounts(t, h, tok, 0, 7)
}

// 細目3: 他人の行は数えない。
func TestActionCountsIgnoresOthers(t *testing.T) {
	h := setupConfig(t, nil)
	me := uuid.New()
	tok := token(t, me)
	someone := uuid.New()
	coach := uuid.New()

	seedActionReview(t, someone, coach, "needs_info")
	seedActionReview(t, someone, coach, "awaiting_payment")
	seedActionReview(t, someone, coach, "in_review")
	seedActionSupport(t, someone, coach, "proposed")
	seedActionSupport(t, someone, uuid.New(), "requested")
	seedActionContract(t, someone, coach, "accepted")
	seedActionContract(t, someone, uuid.New(), "applied")
	seedActionConsultation(t, someone, coach, "offered", "none", false)
	wantActionCounts(t, h, tok, 0, 0)

	// 自分の行を1件だけ入れると、その分だけ増える。
	seedActionSupport(t, me, uuid.New(), "proposed")
	wantActionCounts(t, h, tok, 1, 0)
	if code, _, _ := getActionCounts(t, h, token(t, someone)); code != http.StatusOK {
		t.Fatalf("someone %d", code)
	}
	wantActionCounts(t, h, token(t, someone), 4, 0)
	wantActionCounts(t, h, token(t, coach), 0, 3)
}

// 細目4・5: コーチの行が無い利用者は coach_count=0 で 200。未ログインは 401。応答は2キーの整数。
func TestActionCountsNoCoachRowsAndAuth(t *testing.T) {
	h := setupConfig(t, nil)
	if rec := doJSON(t, h, http.MethodGet, "/v1/me/coaching/action-counts", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous %d %s", rec.Code, rec.Body.String())
	}
	me := uuid.New()
	tok := token(t, me)
	seedActionSupport(t, me, uuid.New(), "proposed")

	code, got, raw := getActionCounts(t, h, tok)
	if code != http.StatusOK || got.PlayerCount == nil || got.CoachCount == nil || *got.PlayerCount != 1 || *got.CoachCount != 0 {
		t.Fatalf("%d %+v", code, got)
	}
	if len(raw) != 2 {
		t.Fatalf("keys %v", raw)
	}
	for _, k := range []string{"player_count", "coach_count"} {
		v, ok := raw[k]
		if !ok {
			t.Fatalf("missing %s", k)
		}
		var n int64
		if err := json.Unmarshal(v, &n); err != nil || strings.ContainsAny(string(v), ".\"") {
			t.Fatalf("%s is not an integer: %s", k, v)
		}
	}
	// 用事が何も無い利用者は 0 と 0。
	wantActionCounts(t, h, token(t, uuid.New()), 0, 0)
}
