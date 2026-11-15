package httpx_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// #748: 精算まで終えた1ヶ月プランの契約があっても退会でき、取引条件の明示
// （support_contract_statements）は行ごと残る（契約への参照だけが外れる）。
// 運営者決定（2026-11-15）で CASCADE ではなく SET NULL（migration 000174）。

// statementRow は、取引条件の明示の1行。契約・コーチへの参照は退会で外れるので、
// 行の追跡は id で行い、参照は NULL 可で読む。
type statementRow struct {
	id         string
	contractID *string
	coachID    *string
	body       string
	issuedAt   time.Time
}

func statementByContract(t *testing.T, contract string) statementRow {
	t.Helper()
	var s statementRow
	queryRowArgs(t, `SELECT id::text, contract_id::text, coach_id::text, body::text, issued_at
		FROM support_contract_statements WHERE contract_id=$1`, []any{contract},
		&s.id, &s.contractID, &s.coachID, &s.body, &s.issuedAt)
	if s.id == "" || s.body == "" {
		t.Fatalf("no statement recorded for contract %s", contract)
	}
	if s.contractID == nil || *s.contractID != contract {
		t.Fatalf("statement not attached to contract %s", contract)
	}
	return s
}

func statementByID(t *testing.T, id string) statementRow {
	t.Helper()
	var s statementRow
	queryRowArgs(t, `SELECT id::text, contract_id::text, coach_id::text, body::text, issued_at
		FROM support_contract_statements WHERE id=$1`, []any{id},
		&s.id, &s.contractID, &s.coachID, &s.body, &s.issuedAt)
	if s.id == "" {
		t.Fatalf("statement %s missing", id)
	}
	return s
}

// 細目1: コーチ退会で statement が残る。contract_id は NULL、coach_id はコーチの
// プロフィール削除で NULL、body と issued_at は退会前と一致。契約行自体は消える。
func TestWithdrawCoachWithSettledContractKeepsStatement(t *testing.T) {
	h, _ := setupStripe(t)
	_, tc, coach, contract := settledCoachFixture(t, h)
	before := statementByContract(t, contract)
	if before.coachID == nil || *before.coachID != coach {
		t.Fatalf("statement coach %v (want %s)", before.coachID, coach)
	}
	if !strings.Contains(before.body, "取引条件の明示") {
		t.Fatalf("statement body does not look like the disclosure: %s", before.body)
	}

	withdrawExpect(t, h, tc, http.StatusOK)

	if contractExists(t, contract) {
		t.Fatal("contract remains after coach withdrawal")
	}
	after := statementByID(t, before.id)
	if after.contractID != nil {
		t.Fatalf("statement still references the deleted contract: %s", *after.contractID)
	}
	if after.coachID != nil {
		t.Fatalf("statement still references the deleted coach profile: %s", *after.coachID)
	}
	if after.body != before.body {
		t.Fatalf("statement body changed\nbefore %s\nafter  %s", before.body, after.body)
	}
	if !after.issuedAt.Equal(before.issuedAt) {
		t.Fatalf("statement issued_at changed %v -> %v", before.issuedAt, after.issuedAt)
	}
}

// 細目2: 選手退会で statement が残る。contract_id は NULL、body と issued_at は
// 退会前と一致、coach_id はコーチが残っているため退会前の値のまま。
func TestWithdrawAthleteWithSettledContractKeepsStatement(t *testing.T) {
	h, _ := setupStripe(t)
	admin := adminToken(t, h)
	ta, _, contract := paidConsultContract(t, h, 2, 10)
	coach := contractCoach(t, contract)
	supportEnd(t, h, admin, contract, map[string]any{"cause": "player"})
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var status, state string
	queryRowArgs(t, `SELECT status, fulfillment_state FROM coaching_contracts WHERE id=$1`, []any{contract}, &status, &state)
	if status != "refunded" || state != "settled" {
		t.Fatalf("contract %s/%s", status, state)
	}
	before := statementByContract(t, contract)
	if before.coachID == nil || *before.coachID != coach {
		t.Fatalf("statement coach %v (want %s)", before.coachID, coach)
	}

	withdrawAccount(t, h, ta)

	if contractExists(t, contract) {
		t.Fatal("contract remains after athlete withdrawal")
	}
	after := statementByID(t, before.id)
	if after.contractID != nil {
		t.Fatalf("statement still references the deleted contract: %s", *after.contractID)
	}
	if after.coachID == nil || *after.coachID != coach {
		t.Fatalf("statement coach %v (want %s to remain)", after.coachID, coach)
	}
	if after.body != before.body {
		t.Fatalf("statement body changed\nbefore %s\nafter  %s", before.body, after.body)
	}
	if !after.issuedAt.Equal(before.issuedAt) {
		t.Fatalf("statement issued_at changed %v -> %v", before.issuedAt, after.issuedAt)
	}
}

// 細目4: 進行中の契約があるあいだは従来どおり 409 で止まり、statement も契約も消えない。
func TestWithdrawBlockedWhileSupportContractOpenKeepsStatement(t *testing.T) {
	h, _ := setupStripe(t)
	ta, tc, contract := paidConsultContract(t, h, 0, 30)
	execSQLArgs(t, `UPDATE coaching_contracts SET
		starts_at=(date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + time '09:00') AT TIME ZONE 'Asia/Tokyo',
		competition_date=date_trunc('month', now() AT TIME ZONE 'Asia/Tokyo')::date - 10 + 29 WHERE id=$1`, contract)
	execSQLArgs(t, `INSERT INTO coaching_messages(id,contract_id,sender_role,body) VALUES(gen_random_uuid(),$1,'coach','よろしくお願いします')`, contract)
	if r := tick(t, h, "tick-secret"); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	markCoachEarningsPaid(t, contractCoach(t, contract))
	before := statementByContract(t, contract)

	for _, tok := range []string{ta, tc} {
		rec := doJSON(t, h, http.MethodDelete, "/v1/me", tok, map[string]string{"confirm": "退会する"})
		if rec.Code != http.StatusConflict {
			t.Fatalf("withdraw with open contract %d %s", rec.Code, rec.Body.String())
		}
	}
	if !contractExists(t, contract) {
		t.Fatal("contract deleted by blocked withdrawal")
	}
	after := statementByContract(t, contract)
	if after.id != before.id || after.body != before.body || !after.issuedAt.Equal(before.issuedAt) {
		t.Fatalf("statement changed by blocked withdrawal\nbefore %+v\nafter %+v", before, after)
	}
}
