package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/competitions"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 写真が取れなかった回でも、前回取れていた写真を空で上書きしない。
func TestStoreMajorHostsKeepsPreviousImageWhenFetchFails(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	if err := config.RequireLocalDatabase(dsn); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// kind は制約で決まっているので、本物の行とぶつからない遠い将来の年だけを使う。
	const kind = "olympics"
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM major_hosts WHERE kind = $1 AND year >= 2090`, kind) })
	host := func(year int, city, image string) competitions.MajorHost {
		return competitions.MajorHost{
			Kind: kind, Year: year, Name: "大会", City: city, Country: "日本", Venue: "競技場",
			StartsOn: time.Date(2090, 8, 1, 0, 0, 0, 0, time.UTC), EndsOn: time.Date(2090, 8, 5, 0, 0, 0, 0, time.UTC),
			ScheduleLabel: "2090年8月", PageURL: "https://example.com/", ImageURL: image,
		}
	}
	image := func(year int) string {
		var out string
		if err := pool.QueryRow(ctx, `SELECT image_url FROM major_hosts WHERE kind = $1 AND year = $2`, kind, year).Scan(&out); err != nil {
			t.Fatalf("year %d: %v", year, err)
		}
		return out
	}
	if err := storeMajorHosts(ctx, pool, kind, []competitions.MajorHost{host(2090, "東京", "https://img/a.jpg"), host(2091, "大阪", "https://img/b.jpg")}); err != nil {
		t.Fatal(err)
	}
	// 2回目は写真が取れず、2091年は取り込みからも外れる。同じ開催地の2090年は前回の写真を残す。
	if err := storeMajorHosts(ctx, pool, kind, []competitions.MajorHost{host(2090, "東京", "")}); err != nil {
		t.Fatal(err)
	}
	if got := image(2090); got != "https://img/a.jpg" {
		t.Fatalf("image kept = %q", got)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM major_hosts WHERE kind = $1 AND year = 2091`, kind).Scan(&n); err != nil || n != 0 {
		t.Fatalf("dropped year still stored: %d %v", n, err)
	}
	// 開催地が変わったときは、前の開催地の写真を引き継がない。
	if err := storeMajorHosts(ctx, pool, kind, []competitions.MajorHost{host(2090, "福岡", "")}); err != nil {
		t.Fatal(err)
	}
	if got := image(2090); got != "" {
		t.Fatalf("image for a different city = %q", got)
	}
}
