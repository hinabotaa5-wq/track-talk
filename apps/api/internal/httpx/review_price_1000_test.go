package httpx_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// 価格改定（本体 #983）：migration 000189 の up で6枚が5,000円・単発の既定値が1,000円になり、down で元に戻る。
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
	check := func(wantSix int, wantDefault string) {
		t.Helper()
		var six int
		var def string
		if err := tx.QueryRow(ctx, `SELECT amount_yen FROM coaching_ticket_products WHERE id = 'six'`).Scan(&six); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT column_default FROM information_schema.columns WHERE table_name = 'review_requests' AND column_name = 'base_price_yen'`).Scan(&def); err != nil {
			t.Fatal(err)
		}
		if six != wantSix || def != wantDefault {
			t.Fatalf("six=%d default=%s, want %d %s", six, def, wantSix, wantDefault)
		}
	}
	check(5000, "1000")
	run("down")
	check(2500, "500")
	run("up")
	check(5000, "1000")
}
