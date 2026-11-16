package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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
	seedActionSupport(t, uuid.New(), me, "requested")
	seedActionSupport(t, uuid.New(), me, "clarifying")
	wantActionCounts(t, h, tok, 0, 8)

	// 数えないもの。
	seedActionConsultation(t, athlete, me, "answered", "none", false) // 補足依頼なし
	seedActionConsultation(t, athlete, me, "answered", "done", false) // 補足に返事済み
	seedActionConsultation(t, athlete, me, "offered", "none", true)   // 消去済み
	seedActionConsultation(t, athlete, me, "declined", "none", false)
	seedActionSupport(t, uuid.New(), me, "proposed")
	seedActionReview(t, athlete, me, "returned")
	seedActionContract(t, uuid.New(), me, "active")
	wantActionCounts(t, h, tok, 0, 8)
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
