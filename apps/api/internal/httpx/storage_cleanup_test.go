package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func cleanupRequest(h http.Handler, secret string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/internal/storage/cleanup", nil)
	r.Header.Set("X-Storage-Cleanup-Secret", secret)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestStorageCleanupPreservesUserDataAndRecoversFailedDeletion(t *testing.T) {
	now := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	live, orphan, recent, removed, productImage := cloudinary.NewPublicID(), cloudinary.NewPublicID(), cloudinary.NewPublicID(), cloudinary.NewPublicID(), cloudinary.NewPublicID()
	failed := true
	var mu sync.Mutex
	var deleted []string
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			defer mu.Unlock()
			id := r.URL.Query().Get("public_ids[]")
			if failed {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			deleted = append(deleted, id)
			json.NewEncoder(w).Encode(map[string]any{"deleted": map[string]string{id: "deleted"}})
			return
		}
		assets := []cloudinary.StoredAsset{}
		if strings.HasSuffix(r.URL.Path, "/image/upload") {
			for _, id := range []string{live, orphan, recent, removed, productImage} {
				at := now.Add(-8 * 24 * time.Hour)
				if id == recent {
					at = now.Add(-6 * 24 * time.Hour)
				}
				assets = append(assets, cloudinary.StoredAsset{PublicID: id, CreatedAt: at})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"resources": assets})
	}))
	defer cloud.Close()
	h := setupConfig(t, func(c *config.Config) {
		c.StorageCleanupSecret = "storage-secret"
		c.CoachingEnabled = false
		c.Clock = func() time.Time { return now }
		c.CloudName = "demo"
		c.CloudAPIKey = "key"
		c.CloudAPISecret = "secret"
		c.CloudAPIBase = cloud.URL
	})
	author := uuid.New()
	execSQLArgs(t, `INSERT INTO profiles(id,nickname) VALUES($1,$2)`, author, author.String()[:8])
	for _, item := range []struct {
		id      string
		deleted bool
	}{{live, false}, {removed, true}} {
		execSQLArgs(t, `INSERT INTO posts(id,author_id,kind,title,body,cloudinary_public_id,created_at,hidden_at,deleted_at) VALUES(gen_random_uuid(),$1,'text','投稿','残す本文',$2,now()-interval '3 years',now(),CASE WHEN $3 THEN now() ELSE NULL END)`, author, item.id, item.deleted)
	}
	injury := uuid.New()
	execSQLArgs(t, `INSERT INTO posts(id,author_id,kind,title,body) VALUES($1,$2,'injury','ケガ','ケガの経過')`, injury, author)
	execSQLArgs(t, `INSERT INTO injury_updates(id,post_id,stage,body) VALUES(gen_random_uuid(),$1,'resting','残す経過')`, injury)
	execSQLArgs(t, `INSERT INTO sense_notes(id,author_id,body) VALUES(gen_random_uuid(),$1,'残す感覚')`, author)
	execSQLArgs(t, `INSERT INTO race_records(id,author_id,raced_on,event,mark,memo) VALUES(gen_random_uuid(),$1,'2020-01-01','100m',1100,'残す記録')`, author)
	product := uuid.New()
	execSQLArgs(t, `INSERT INTO products(id,kind,brand,name,image_url) VALUES($1,'shoe','cleanup',$2,$3)`, product, product.String(), "https://res.cloudinary.com/demo/image/upload/"+productImage+".jpg")
	execSQLArgs(t, `INSERT INTO product_reviews(id,product_id,author_id,rating,body) VALUES(gen_random_uuid(),$1,$2,5,'残すレビュー')`, product, author)
	t.Cleanup(func() {
		execSQLArgs(t, `DELETE FROM product_reviews WHERE product_id=$1`, product)
		execSQLArgs(t, `DELETE FROM products WHERE id=$1`, product)
	})
	insertAgedNotifications(t, author, 100)
	insertAgedNotifications(t, author, 200)
	execSQLArgs(t, `INSERT INTO competitions(id,name,starts_on,prefecture,season_year,organizer_url,source_url,link_label) VALUES(gen_random_uuid(),'古い大会','2025-10-01','47',2025,'https://example.com','https://example.com','要項'),(gen_random_uuid(),'今期大会','2026-10-01','47',2026,'https://example.com','https://example.com','要項')`)
	execSQLArgs(t, `INSERT INTO domestic_majors(slug,year,name,category,starts_on,ends_on,schedule_label) VALUES('cleanup-past',2025,'古い主要大会','open','2026-03-30','2026-03-31',''),('cleanup-cross',2025,'年度をまたぐ大会','open','2026-03-30','2026-04-02','')`)
	t.Cleanup(func() {
		execSQLArgs(t, `DELETE FROM domestic_majors WHERE slug LIKE 'cleanup-%'`)
		execSQLArgs(t, `DELETE FROM competitions WHERE prefecture='47'`)
	})
	if w := cleanupRequest(h, ""); w.Code != 404 {
		t.Fatalf("unauthenticated %d", w.Code)
	}
	if w := cleanupRequest(h, "storage-secret"); w.Code != 500 {
		t.Fatalf("failure %d %s", w.Code, w.Body.String())
	}
	mu.Lock()
	failed = false
	mu.Unlock()
	w := cleanupRequest(h, "storage-secret")
	if w.Code != 200 {
		t.Fatalf("cleanup %d %s", w.Code, w.Body.String())
	}
	mu.Lock()
	got := strings.Join(deleted, ",")
	mu.Unlock()
	if len(deleted) != 2 || !strings.Contains(got, orphan) || !strings.Contains(got, removed) {
		t.Fatalf("deleted %v", deleted)
	}
	var count int
	for _, table := range []string{"posts", "sense_notes", "race_records", "injury_updates", "product_reviews"} {
		queryRowArgs(t, "SELECT count(*) FROM "+table, nil, &count)
		if count == 0 {
			t.Fatalf("protected table %s cleared", table)
		}
	}
	queryRowArgs(t, `SELECT count(*) FROM posts WHERE author_id=$1`, []any{author}, &count)
	if count != 3 {
		t.Fatalf("posts %d", count)
	}
	if read, unread := countNotificationsOf(t, author); read != 0 || unread != 1 {
		t.Fatalf("notifications %d %d", read, unread)
	}
	queryRowArgs(t, `SELECT count(*) FROM competitions WHERE prefecture='47' AND season_year=2025`, nil, &count)
	if count != 0 {
		t.Fatal("past season remained")
	}
	queryRowArgs(t, `SELECT count(*) FROM domestic_majors WHERE slug='cleanup-cross'`, nil, &count)
	if count != 1 {
		t.Fatal("cross-season event removed")
	}
	pool := openPool(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO posts(id,author_id,kind,title,body,cloudinary_public_id) VALUES(gen_random_uuid(),$1,'text','投稿','古い画面からの再送',$2)`, author, orphan); err == nil {
		t.Fatal("retired file could be attached again")
	}
}

func TestStorageCleanupWaitsForConcurrentPostBeforeDeleting(t *testing.T) {
	id := cloudinary.NewPublicID()
	listed := make(chan struct{}, 1)
	var deleted bool
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
			t.Error("concurrently attached media deleted")
		}
		assets := []cloudinary.StoredAsset{}
		if strings.HasSuffix(r.URL.Path, "/image/upload") {
			assets = append(assets, cloudinary.StoredAsset{PublicID: id, CreatedAt: time.Now().Add(-8 * 24 * time.Hour)})
			listed <- struct{}{}
		}
		json.NewEncoder(w).Encode(map[string]any{"resources": assets})
	}))
	defer cloud.Close()
	h := setupConfig(t, func(c *config.Config) {
		c.StorageCleanupSecret = "secret"
		c.CloudName = "demo"
		c.CloudAPIKey = "key"
		c.CloudAPISecret = "secret"
		c.CloudAPIBase = cloud.URL
	})
	author := uuid.New()
	execSQLArgs(t, `INSERT INTO profiles(id,nickname) VALUES($1,$2)`, author, author.String()[:8])
	pool := openPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `INSERT INTO posts(id,author_id,kind,title,body,cloudinary_public_id) VALUES(gen_random_uuid(),$1,'text','投稿','保存中',$2)`, author, id); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- cleanupRequest(h, "secret") }()
	select {
	case <-listed:
	case <-time.After(5 * time.Second):
		t.Fatal("assets not listed")
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-done:
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not finish")
	}
	if deleted {
		t.Fatal("media deleted")
	}
}
