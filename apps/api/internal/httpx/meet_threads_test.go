package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
	"github.com/hinata/track-talk/apps/api/internal/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 実況スレッドの投稿者になる運営プロフィール。setup の TRUNCATE で消えるので入れ直す。
func seedSystemProfile(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO profiles (id, nickname, nickname_public, is_admin) VALUES ($1, 'TrackTalk', true, false) ON CONFLICT (id) DO NOTHING`,
		httpx.SystemProfileID); err != nil {
		t.Fatal(err)
	}
}

func TestTodayMeetThreadsCreatesOneThreadPerTargetMeet(t *testing.T) {
	// 2026-10-12 正午(JST)に固定する。
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	fixed := time.Date(2026, time.October, 12, 12, 0, 0, 0, jst)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.Clock = func() time.Time { return fixed }
	})
	pool := openPool(t)
	ctx := context.Background()
	seedSystemProfile(t, pool)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM domestic_majors WHERE slug IN ('izumo-ekiden', 'all-japan')`)
		_, _ = pool.Exec(ctx, `DELETE FROM competitions WHERE prefecture = '00' AND name IN ('ダイヤモンドリーグ・ファイナル', 'コンチネンタルツアー・ゴールド')`)
	})

	insertMajor := func(slug, name, starts, ends string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `
			INSERT INTO domestic_majors (slug, year, name, category, venue, starts_on, ends_on, schedule_label, site_url)
			VALUES ($1, 2026, $2, 'open', '会場', $3::date, $4::date, '2026年', 'https://example.jp/')`,
			slug, name, starts, ends); err != nil {
			t.Fatal(err)
		}
	}
	// 今日開催中の国内主要大会と、今日ではない国内主要大会。
	insertMajor("izumo-ekiden", "出雲駅伝", "2026-10-12", "2026-10-12")
	insertMajor("all-japan", "後日の大会", "2026-11-03", "2026-11-03")

	insertComp := func(name, starts string, ends *string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO competitions (id, name, starts_on, ends_on, location, prefecture, season_year, organizer_url, source_url)
			VALUES ($1, $2, $3::date, $4::date, '', '00', 2026, 'https://worldathletics.org/', 'https://worldathletics.org/')`,
			id, name, starts, ends); err != nil {
			t.Fatal(err)
		}
		return id
	}
	// 開催中の featured 国際大会と、開催中だが featured でない国際大会。
	finalID := insertComp("ダイヤモンドリーグ・ファイナル", "2026-10-11", strPtr("2026-10-13"))
	insertComp("コンチネンタルツアー・ゴールド", "2026-10-12", nil)

	// ログイン不要で読める。
	rec := doJSON(t, h, http.MethodGet, "/v1/meet-threads/today", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("today %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Meets []struct {
			MeetKey       string `json:"meet_key"`
			Name          string `json:"name"`
			DateLabel     string `json:"date_label"`
			PostID        string `json:"post_id"`
			CommentCount  int    `json:"comment_count"`
			Href          string `json:"href"`
			MajorSlug     string `json:"major_slug"`
			MajorYear     int    `json:"major_year"`
			CompetitionID string `json:"competition_id"`
		} `json:"meets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Meets) != 2 {
		t.Fatalf("meets = %d, want 2: %s", len(body.Meets), rec.Body.String())
	}
	domestic, intl := body.Meets[0], body.Meets[1]
	if domestic.MeetKey != "m:izumo-ekiden:2026" || domestic.Name != "出雲駅伝" || domestic.DateLabel != "2026年10月12日" {
		t.Fatalf("domestic %+v", domestic)
	}
	if domestic.Href != "/posts/"+domestic.PostID {
		t.Fatalf("domestic href %+v", domestic)
	}
	if intl.MeetKey != "c:00:2026:ダイヤモンドリーグ・ファイナル:2026-10-11" || intl.Name != "ダイヤモンドリーグ・ファイナル" {
		t.Fatalf("intl %+v", intl)
	}
	// 大会タブの一覧と突き合わせるための対応。国内主要大会は slug と年、国際大会は大会の ID。
	if domestic.MajorSlug != "izumo-ekiden" || domestic.MajorYear != 2026 || domestic.CompetitionID != "" {
		t.Fatalf("domestic list link %+v", domestic)
	}
	if intl.CompetitionID != finalID.String() || intl.MajorSlug != "" || intl.MajorYear != 0 {
		t.Fatalf("intl list link %+v", intl)
	}

	// 実況スレッドは運営プロフィールが投稿した kind=meet の投稿。
	var authorID string
	var kind, title string
	if err := pool.QueryRow(ctx, `SELECT author_id::text, kind, title FROM posts WHERE id = $1`, domestic.PostID).
		Scan(&authorID, &kind, &title); err != nil {
		t.Fatal(err)
	}
	if authorID != httpx.SystemProfileID.String() || kind != "meet" {
		t.Fatalf("post author %s kind %s", authorID, kind)
	}
	if want := "出雲駅伝（2026年10月12日）実況"; title != want {
		t.Fatalf("title = %q, want %q", title, want)
	}

	// もう一度叩いても増えない(1大会1スレッド)。
	if rec := doJSON(t, h, http.MethodGet, "/v1/meet-threads/today", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("today again %d %s", rec.Code, rec.Body.String())
	}
	var meetPosts, threadRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM posts WHERE kind = 'meet'`).Scan(&meetPosts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM meet_threads`).Scan(&threadRows); err != nil {
		t.Fatal(err)
	}
	if meetPosts != 2 || threadRows != 2 {
		t.Fatalf("idempotency: meet posts %d threads %d, want 2/2", meetPosts, threadRows)
	}

	// 実況スレッドはホームのフィードに出ない。
	rec = doJSON(t, h, http.MethodGet, "/v1/posts?sort=new", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed %d %s", rec.Code, rec.Body.String())
	}
	var feed struct {
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &feed)
	if feed.Meta.Total != 0 {
		t.Fatalf("meet post leaked into feed, total = %d", feed.Meta.Total)
	}
}

func TestUserCannotCreateMeetPost(t *testing.T) {
	h := setup(t)
	tok := token(t, uuid.New())
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", tok, map[string]any{
		"title": "勝手に実況", "body": "だめ", "kind": "meet",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("user-created meet post = %d, want 422: %s", rec.Code, rec.Body.String())
	}
}

func strPtr(s string) *string { return &s }

// 運営が貼り付けたタイムテーブルが、今日の実況スレッドの「いま実施中」に出る。
func TestAdminMeetSessionsShowNowRunning(t *testing.T) {
	jstZone := time.FixedZone("Asia/Tokyo", 9*60*60)
	fixed := time.Date(2026, time.October, 12, 10, 45, 0, 0, jstZone)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.Clock = func() time.Time { return fixed }
	})
	pool := openPool(t)
	ctx := context.Background()
	seedSystemProfile(t, pool)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM domestic_majors WHERE slug = 'izumo-ekiden'`)
		_, _ = pool.Exec(ctx, `DELETE FROM meet_sessions`)
	})
	if _, err := pool.Exec(ctx, `
		INSERT INTO domestic_majors (slug, year, name, category, venue, starts_on, ends_on, schedule_label, site_url)
		VALUES ('izumo-ekiden', 2026, '出雲駅伝', 'open', '会場', '2026-10-12', '2026-10-12', '2026年', 'https://example.jp/')`); err != nil {
		t.Fatal(err)
	}

	admin, user := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profiles (id, is_admin) VALUES ($1, true), ($2, false)`, admin, user); err != nil {
		t.Fatal(err)
	}
	ta, tu := token(t, admin), token(t, user)
	const key = "m:izumo-ekiden:2026"
	put := func(tok, meetKey, csv string) int {
		return doJSON(t, h, http.MethodPut, "/v1/admin/meet-sessions", tok, map[string]string{"meet_key": meetKey, "csv": csv}).Code
	}

	if code := put(tu, key, "2026-10-12 10:30,100m"); code != http.StatusForbidden {
		t.Fatalf("non-admin %d", code)
	}
	if code := put(ta, "m:unknown:2026", "2026-10-12 10:30,100m"); code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown meet %d", code)
	}
	if code := put(ta, key, "2026-10-12 10:30,100m,予選,宇宙人"); code != http.StatusUnprocessableEntity {
		t.Fatalf("bad row %d", code)
	}
	if code := put(ta, key, "開始,種目,ラウンド,性別\n2026-10-12 10:30,100m,予選,女子\n2026-10-12 11:00,200m,予選,女子"); code != http.StatusOK {
		t.Fatalf("put %d", code)
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/meet-sessions", ta, nil)
	var list struct {
		Meets []struct {
			MeetKey      string `json:"meet_key"`
			SessionCount int    `json:"session_count"`
		} `json:"meets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Meets) != 1 || list.Meets[0].MeetKey != key || list.Meets[0].SessionCount != 2 {
		t.Fatalf("admin list %d %s", rec.Code, rec.Body.String())
	}

	today := func() []string {
		rec := doJSON(t, h, http.MethodGet, "/v1/meet-threads/today", "", nil)
		var body struct {
			Meets []struct {
				NowRunning []string `json:"now_running"`
			} `json:"meets"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Meets) != 1 {
			t.Fatalf("today %d %s", rec.Code, rec.Body.String())
		}
		return body.Meets[0].NowRunning
	}
	if got := today(); len(got) != 1 || got[0] != "女子100m予選" {
		t.Fatalf("now running %v", got)
	}

	// 貼り直すと置き換わる。空にすると消える。
	if code := put(ta, key, ""); code != http.StatusOK {
		t.Fatalf("clear %d", code)
	}
	if got := today(); len(got) != 0 {
		t.Fatalf("cleared %v", got)
	}
	// 実施中が無いときも、null ではなく空の配列で返す。
	if body := doJSON(t, h, http.MethodGet, "/v1/meet-threads/today", "", nil).Body.String(); !strings.Contains(body, `"now_running":[]`) {
		t.Fatalf("now_running should be an empty array: %s", body)
	}
}

// 3月に始まって4月にまたがる国内主要大会は、4月になっても開催中なら実況スレッドが出る。
func TestTodayMeetThreadsIncludesMajorSpanningSeasonStart(t *testing.T) {
	jstZone := time.FixedZone("Asia/Tokyo", 9*60*60)
	fixed := time.Date(2027, time.April, 1, 12, 0, 0, 0, jstZone)
	h := setupConfig(t, func(cfg *config.Config) {
		cfg.Clock = func() time.Time { return fixed }
	})
	pool := openPool(t)
	ctx := context.Background()
	seedSystemProfile(t, pool)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM domestic_majors WHERE slug = 'spring-spanning'`) })
	if _, err := pool.Exec(ctx, `
		INSERT INTO domestic_majors (slug, year, name, category, venue, starts_on, ends_on, schedule_label, site_url)
		VALUES ('spring-spanning', 2027, '春またぎ大会', 'open', '会場', '2027-03-30', '2027-04-02', '2027年', 'https://example.jp/')`); err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, h, http.MethodGet, "/v1/meet-threads/today", "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "春またぎ大会") {
		t.Fatalf("today %d %s", rec.Code, rec.Body.String())
	}
}
