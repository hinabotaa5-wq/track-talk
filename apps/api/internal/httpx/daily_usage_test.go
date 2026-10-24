package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDailyUsageKeepsSuccessfulCountsAfterDeletion(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "TRUNCATE daily_usage_counts"); err != nil {
		t.Fatal(err)
	}
	adminID, userID := uuid.New(), uuid.New()
	admin, user := token(t, adminID), token(t, userID)
	for _, tok := range []string{admin, user, user} {
		if rec := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil); rec.Code != 200 {
			t.Fatalf("profile: %d %s", rec.Code, rec.Body.String())
		}
	}
	setAdmin(t, adminID)
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", "", nil); rec.Code != 401 {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", user, nil); rec.Code != 403 {
		t.Fatalf("member: %d", rec.Code)
	}
	created := doJSON(t, h, http.MethodPost, "/v1/posts", user, map[string]string{"body": "集計に含めない秘密の本文"})
	if created.Code != 201 {
		t.Fatalf("post: %d %s", created.Code, created.Body.String())
	}
	var post struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO posts (id, author_id, title, body, kind) VALUES ($1, $2, '大会スレッド', '', 'meet')", uuid.New(), adminID); err != nil {
		t.Fatal(err)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/posts/"+post.ID+"/comments", admin, map[string]string{"body": "秘密のコメント"}); rec.Code != 201 {
		t.Fatalf("comment: %d %s", rec.Code, rec.Body.String())
	}
	product := doJSON(t, h, http.MethodPost, "/v1/admin/products", admin, map[string]string{"kind": "shoe", "brand": "DailyTest", "name": uuid.NewString()})
	if product.Code != 201 {
		t.Fatalf("product: %d %s", product.Code, product.Body.String())
	}
	var productID struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(product.Body.Bytes(), &productID); err != nil {
		t.Fatal(err)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/products/"+productID.ID+"/reviews", user, map[string]any{"body": "秘密のレビュー", "rating": 4, "event": "100m"}); rec.Code != 201 {
		t.Fatalf("review: %d %s", rec.Code, rec.Body.String())
	}
	// 同じいいねの再送は加算せず、取り消してから押し直した分は加算する。
	for _, method := range []string{http.MethodPost, http.MethodPost, http.MethodDelete, http.MethodPost} {
		rec := doJSON(t, h, method, "/v1/posts/"+post.ID+"/likes", admin, nil)
		if rec.Code != 204 {
			t.Fatalf("like: %d %s", rec.Code, rec.Body.String())
		}
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/sense-notes", user, map[string]string{"body": "秘密のメモ", "event": "100m"}); rec.Code != 201 {
		t.Fatalf("note: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/me/race-records", user, map[string]any{"raced_on": "2026-05-03", "event": "100m", "mark": -1}); rec.Code != 422 {
		t.Fatalf("invalid record: %d", rec.Code)
	}
	record := doJSON(t, h, http.MethodPost, "/v1/me/race-records", user, map[string]any{"raced_on": "2026-05-03", "event": "100m", "mark": 1200})
	if record.Code != 201 {
		t.Fatalf("record: %d %s", record.Code, record.Body.String())
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/posts/"+post.ID, user, map[string]string{"body": "編集後"}); rec.Code != 200 {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/posts/"+post.ID, user, nil); rec.Code != 204 {
		t.Fatalf("delete post: %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/me", user, map[string]string{"confirm": "退会する"}); rec.Code != 200 {
		t.Fatalf("withdraw: %d %s", rec.Code, rec.Body.String())
	}

	// ロールバックされた書き込みは集計も巻き戻る。
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO profiles (id) VALUES ($1)", uuid.New()); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/stats", admin, nil)
	if rec.Code != 200 {
		t.Fatalf("stats: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		StartedAt string `json:"daily_started_at"`
		Daily     []struct {
			Day            string `json:"day"`
			Signups        int    `json:"signups"`
			Posts          int    `json:"posts"`
			Comments       int    `json:"comments"`
			Likes          int    `json:"likes"`
			SenseNotes     int    `json:"sense_notes"`
			RaceRecords    int    `json:"race_records"`
			Deletions      int    `json:"deletions"`
			ProductReviews int    `json:"product_reviews"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.StartedAt == "" || len(out.Daily) == 0 {
		t.Fatalf("missing daily stats: %s", rec.Body.String())
	}
	d := out.Daily[0]
	if d.Day != time.Now().In(jst.Zone).Format(time.DateOnly) || d.Signups != 2 || d.Posts != 1 || d.Comments != 1 || d.Likes != 2 || d.SenseNotes != 1 || d.RaceRecords != 1 || d.Deletions != 1 || d.ProductReviews != 1 {
		t.Fatalf("daily stats: %+v", d)
	}
	for _, secret := range []string{"秘密", userID.String(), adminID.String(), post.ID} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("stats leaked content: %s", secret)
		}
	}
	// 履歴のない日も欠落ではなく0で返す。
	tomorrow := time.Now().In(jst.Zone).AddDate(0, 0, 1)
	rows, err := db.New(pool).AdminDailyUsage(ctx, db.AdminDailyUsageParams{Since: tomorrow, Until: tomorrow.AddDate(0, 0, 1)})
	if err != nil || len(rows) != 2 || rows[0].Signups != 0 || rows[1].Posts != 0 {
		t.Fatalf("zero days: %+v %v", rows, err)
	}
}

func TestDailyUsageConcurrentWritesAndTokyoDate(t *testing.T) {
	_ = setup(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "TRUNCATE daily_usage_counts"); err != nil {
		t.Fatal(err)
	}
	const writes = 16
	var wg sync.WaitGroup
	errors := make(chan error, writes)
	for i := 0; i < writes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := pool.Begin(ctx)
			if err != nil {
				errors <- err
				return
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, "SET LOCAL TIME ZONE 'UTC'"); err == nil {
				_, err = tx.Exec(ctx, "INSERT INTO profiles (id) VALUES ($1)", uuid.New())
			}
			if err == nil {
				err = tx.Commit(ctx)
			}
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var day string
	var count int
	if err := pool.QueryRow(ctx, "SELECT day::text, count FROM daily_usage_counts WHERE metric = 'signup'").Scan(&day, &count); err != nil {
		t.Fatal(err)
	}
	if count != writes || day != time.Now().In(jst.Zone).Format(time.DateOnly) {
		t.Fatalf("day=%s count=%d", day, count)
	}
}
