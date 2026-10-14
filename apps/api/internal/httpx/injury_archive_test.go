package httpx_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type archiveRow struct {
	BodyPart, Place, Situation, Timeline, Posts, Review string
	InjuredMonth                                        string
}

func readArchives(t *testing.T) []archiveRow {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `SELECT body_part, place, situation, stage_timeline::text, posts::text, review_status, to_char(injured_month, 'YYYY-MM-DD') FROM injury_archives ORDER BY body_part`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []archiveRow
	for rows.Next() {
		var r archiveRow
		if err := rows.Scan(&r.BodyPart, &r.Place, &r.Situation, &r.Timeline, &r.Posts, &r.Review, &r.InjuredMonth); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func clearArchives(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tracktalk:tracktalk@localhost:5432/tracktalk?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE injury_archives`); err != nil {
		t.Fatal(err)
	}
}

func TestWithdrawKeepsInjuryRecordsOnlyWithConsent(t *testing.T) {
	h := setup(t)
	clearArchives(t)

	build := func() string {
		tok := token(t, uuid.New())
		body := joinBody("resting")
		body["place"] = "河川敷 連絡 runner@example.com"
		if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/join", tok, body); rec.Code != http.StatusOK {
			t.Fatalf("join %d %s", rec.Code, rec.Body.String())
		}
		if rec := doJSON(t, h, http.MethodPatch, "/v1/injury/groups/knee/me", tok, map[string]any{"stage": "jogging"}); rec.Code != http.StatusOK {
			t.Fatalf("stage %d", rec.Code)
		}
		if rec := doJSON(t, h, http.MethodPost, "/v1/injury/groups/knee/posts", tok, map[string]any{
			"kind": "injury", "title": "ジョグ再開", "body": "090-1234-5678 に連絡ください。5分走れた", "body_part": "knee",
		}); rec.Code != http.StatusCreated {
			t.Fatalf("post %d %s", rec.Code, rec.Body.String())
		}
		return tok
	}

	// 同意しない退会では、何も残らない。
	declined := build()
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", declined, map[string]any{"confirm": "退会する"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	if rows := readArchives(t); len(rows) != 0 {
		t.Fatalf("archived without consent: %+v", rows)
	}

	// 同意した退会では、個人が特定できそうな記述を取り除いて、匿名で残る。
	agreed := build()
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", agreed, map[string]any{"confirm": "退会する", "keep_injury_anonymized": true}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body.String())
	}
	rows := readArchives(t)
	if len(rows) != 1 {
		t.Fatalf("archives %+v", rows)
	}
	got := rows[0]
	if got.BodyPart != "knee" || got.Review != "pending" || !strings.HasSuffix(got.InjuredMonth, "-01") {
		t.Fatalf("archive %+v", got)
	}
	if strings.Contains(got.Place, "runner@example.com") || !strings.Contains(got.Place, "河川敷") {
		t.Fatalf("place not scrubbed: %q", got.Place)
	}
	if strings.Contains(got.Posts, "090-1234-5678") || !strings.Contains(got.Posts, "5分走れた") {
		t.Fatalf("posts not scrubbed: %s", got.Posts)
	}
	if !strings.Contains(got.Timeline, `"resting"`) || !strings.Contains(got.Timeline, `"jogging"`) {
		t.Fatalf("timeline %s", got.Timeline)
	}

	// 退会したら、グループの参加と投稿そのものは消える。
	if rec := doJSON(t, h, http.MethodGet, "/v1/injury/groups/knee", agreed, nil); rec.Code == http.StatusOK {
		t.Fatalf("group still readable after withdrawal")
	}
}
