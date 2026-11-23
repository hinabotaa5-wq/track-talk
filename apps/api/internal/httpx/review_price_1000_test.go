package httpx_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// 価格改定（本体 #983）：migration 000189 の up で6枚が5,000円・単発の既定値が1,000円・失効後返金の上限が1枚1,000円になり、down で元に戻る。
func TestReviewPrice1000MigrationRoundTrip(t *testing.T) {
	_ = setup(t)
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

	run := func(direction string) {
		t.Helper()
		source, err := os.ReadFile("../../migrations/000189_review_price_1000." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, string(source)); err != nil {
			t.Fatal(err)
		}
	}
	check := func(wantSix int, wantDefault string, wantCap string) {
		t.Helper()
		var six int
		var def, cap string
		if err := tx.QueryRow(ctx, `SELECT amount_yen FROM coaching_ticket_products WHERE id = 'six'`).Scan(&six); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT column_default FROM information_schema.columns WHERE table_name = 'review_requests' AND column_name = 'base_price_yen'`).Scan(&def); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'coaching_ticket_purchases_refund_cap_check'`).Scan(&cap); err != nil {
			t.Fatal(err)
		}
		if six != wantSix || def != wantDefault || !strings.Contains(cap, wantCap) {
			t.Fatalf("six=%d default=%s cap=%s, want %d %s %s", six, def, cap, wantSix, wantDefault, wantCap)
		}
	}
	check(5000, "1000", "quantity * 1000")
	run("down")
	check(2500, "500", "quantity * 500")
	run("up")
	check(5000, "1000", "quantity * 1000")
}
