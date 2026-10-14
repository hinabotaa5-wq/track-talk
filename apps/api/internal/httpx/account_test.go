package httpx_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/config"
)

func TestDeleteAccountRemovesUserData(t *testing.T) {
	h := setup(t)
	authorID := uuid.New()
	otherID := uuid.New()
	author, other := token(t, authorID), token(t, otherID)

	rec := doJSON(t, h, http.MethodDelete, "/v1/me", "", map[string]string{"confirm": "退会する"})
	if rec.Code != 401 {
		t.Fatalf("anon %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodDelete, "/v1/me", author, map[string]string{"confirm": "やめる"})
	if rec.Code != 422 {
		t.Fatalf("confirm %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", author, map[string]any{
		"title": "自分の投稿", "body": "退会で消える本文です",
	})
	if rec.Code != 201 {
		t.Fatalf("author post %d %s", rec.Code, rec.Body.String())
	}
	var authorPost struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &authorPost)

	rec = doJSON(t, h, http.MethodPost, "/v1/posts", other, map[string]any{
		"title": "他人の投稿", "body": "残る本文です",
	})
	if rec.Code != 201 {
		t.Fatalf("other post %d %s", rec.Code, rec.Body.String())
	}
	var otherPost struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &otherPost)

	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+otherPost.ID+"/comments", author, map[string]string{"body": "残ってはいけないコメント"})
	if rec.Code != 201 {
		t.Fatalf("comment %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+authorPost.ID+"/comments", other, map[string]string{"body": "投稿ごと消えるコメント"})
	if rec.Code != 201 {
		t.Fatalf("reply %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+otherPost.ID+"/likes", author, nil)
	if rec.Code != 204 {
		t.Fatalf("like %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+otherPost.ID+"/saves", author, nil)
	if rec.Code != 204 {
		t.Fatalf("save %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/reports", author, map[string]string{
		"target_type": "post", "target_id": otherPost.ID, "reason": "spam", "detail": "",
	})
	if rec.Code != 201 {
		t.Fatalf("report %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/me/inquiries", author, map[string]string{
		"category": "other", "subject": "退会前", "body": "問い合わせ本文",
	})
	if rec.Code != 201 {
		t.Fatalf("inquiry %d %s", rec.Code, rec.Body.String())
	}

	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	starts := time.Now().In(loc).Add(72 * time.Hour)
	starts = time.Date(starts.Year(), starts.Month(), starts.Day(), 18, 0, 0, 0, loc)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts", other, map[string]any{
		"kind": "practice", "body": "残る練習です", "venue": "競技場",
		"starts_at": starts.Format(time.RFC3339), "event": "100m", "capacity": 2,
	})
	if rec.Code != 201 {
		t.Fatalf("practice %d %s", rec.Code, rec.Body.String())
	}
	var practice struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &practice)
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practice.ID+"/practice/applications", author, nil)
	if rec.Code != 201 {
		t.Fatalf("apply %d %s", rec.Code, rec.Body.String())
	}
	var applied struct {
		Practice struct {
			MyApplicationID string `json:"my_application_id"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &applied)
	applicationID := applied.Practice.MyApplicationID
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practice.ID+"/practice/applications/"+applicationID+"/accept", other, nil)
	if rec.Code != 200 {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practice.ID+"/practice/dissolve", other, nil)
	if rec.Code != 200 {
		t.Fatalf("dissolve %d %s", rec.Code, rec.Body.String())
	}
	rating := map[string]any{"score": "good", "comment": "時間通りでした"}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practice.ID+"/practice/applications/"+applicationID+"/rating", author, rating)
	if rec.Code != 201 {
		t.Fatalf("author rate %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/v1/posts/"+practice.ID+"/practice/applications/"+applicationID+"/rating", other, map[string]any{"score": "ok", "comment": ""})
	if rec.Code != 200 && rec.Code != 201 {
		t.Fatalf("other rate %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodDelete, "/v1/me", author, map[string]string{"confirm": "退会する"})
	if rec.Code != 200 {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	var deleted struct {
		AuthDeleted bool `json:"auth_deleted"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &deleted)
	if deleted.AuthDeleted {
		t.Fatal("auth user deleted without service role key")
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+authorPost.ID, other, nil)
	if rec.Code != 404 {
		t.Fatalf("author post remains %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+otherPost.ID, other, nil)
	if rec.Code != 200 {
		t.Fatalf("other post %d %s", rec.Code, rec.Body.String())
	}
	var kept struct {
		LikeCount    int `json:"like_count"`
		CommentCount int `json:"comment_count"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &kept)
	if kept.LikeCount != 0 || kept.CommentCount != 0 {
		t.Fatalf("counts %+v", kept)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+otherPost.ID+"/comments", other, nil)
	var comments struct {
		Comments []any `json:"comments"`
		Meta     struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &comments)
	if rec.Code != 200 || comments.Meta.Total != 0 || len(comments.Comments) != 0 {
		t.Fatalf("comments %d %+v", rec.Code, comments)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+practice.ID, other, nil)
	var practiceLeft struct {
		Practice struct {
			Applications []any `json:"applications"`
		} `json:"practice"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &practiceLeft)
	if rec.Code != 200 || len(practiceLeft.Practice.Applications) != 0 {
		t.Fatalf("practice %d %+v", rec.Code, practiceLeft.Practice)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/practice-reputation", other, nil)
	var reputation struct {
		Good int `json:"good"`
		Ok   int `json:"ok"`
		Bad  int `json:"bad"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &reputation)
	if rec.Code != 200 || reputation.Good != 0 || reputation.Ok != 0 || reputation.Bad != 0 {
		t.Fatalf("reputation %d %+v", rec.Code, reputation)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/me/posts", author, nil)
	var mine struct {
		Posts []any `json:"posts"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &mine)
	if rec.Code != 200 || len(mine.Posts) != 0 {
		t.Fatalf("my posts %d %+v", rec.Code, mine)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/saves", author, nil)
	var saves struct {
		Posts []any `json:"posts"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &saves)
	if rec.Code != 200 || len(saves.Posts) != 0 {
		t.Fatalf("saves %d %+v", rec.Code, saves)
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/me/inquiries", author, nil)
	var inquiries struct {
		Inquiries []any `json:"inquiries"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &inquiries)
	if rec.Code != 200 || len(inquiries.Inquiries) != 0 {
		t.Fatalf("inquiries %d %+v", rec.Code, inquiries)
	}
}

func TestDeleteAccountKeepsPurgeWhenAuthDeleteFails(t *testing.T) {
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method %s", r.Method)
		}
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(authSrv.Close)

	h := setupConfig(t, func(cfg *config.Config) {
		cfg.SupabaseURL = authSrv.URL
		cfg.SupabaseServiceRoleKey = "service-role-test"
	})
	authorID := uuid.New()
	author := token(t, authorID)
	rec := doJSON(t, h, http.MethodPost, "/v1/posts", author, map[string]any{
		"title": "消える投稿", "body": "認証削除に失敗しても投稿は残しません",
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	var logs strings.Builder
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	rec = doJSON(t, h, http.MethodDelete, "/v1/me", author, map[string]string{"confirm": "退会する"})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}
	var deleted struct {
		AuthDeleted bool `json:"auth_deleted"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &deleted)
	if deleted.AuthDeleted {
		t.Fatal("auth user deleted after supabase error")
	}
	if !strings.Contains(logs.String(), authorID.String()) || !strings.Contains(logs.String(), "auth user remains after account purge") {
		t.Fatalf("retry log %s", logs.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("purge rolled back %d %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteAccountRejectsStaleLogin(t *testing.T) {
	h := setup(t)
	freshID := uuid.New()
	staleID := uuid.New()
	missingID := uuid.New()
	fresh := tokenWithAMR(t, freshID, time.Now())
	stale := tokenAt(t, staleID, time.Now().Add(-6*time.Minute))
	missing := tokenAt(t, missingID, time.Time{})

	for _, item := range []struct {
		name string
		id   uuid.UUID
		tok  string
		want int
	}{
		{name: "amr", id: freshID, tok: fresh, want: http.StatusOK},
		{name: "stale", id: staleID, tok: stale, want: http.StatusUnauthorized},
		{name: "missing", id: missingID, tok: missing, want: http.StatusUnauthorized},
	} {
		t.Run(item.name, func(t *testing.T) {
			rec := doJSON(t, h, http.MethodPost, "/v1/posts", item.tok, map[string]any{
				"title": item.name, "body": "退会できない投稿です",
			})
			if rec.Code != http.StatusCreated {
				t.Fatalf("create %d %s", rec.Code, rec.Body.String())
			}
			var created struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &created)

			rec = doJSON(t, h, http.MethodDelete, "/v1/me", item.tok, map[string]string{"confirm": "退会する"})
			if rec.Code != item.want {
				t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
			}
			if item.want == http.StatusUnauthorized {
				var body struct {
					Error struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				if body.Error.Code != "reauth_required" || body.Error.Message != "退会する前にもう一度ログインしてください" {
					t.Fatalf("body %+v", body)
				}
			}
			rec = doJSON(t, h, http.MethodGet, "/v1/posts/"+created.ID, "", nil)
			wantGet := http.StatusOK
			if item.want == http.StatusOK {
				wantGet = http.StatusNotFound
			}
			if rec.Code != wantGet {
				t.Fatalf("post %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func tokenWithAMR(t *testing.T, id uuid.UUID, authTime time.Time) string {
	t.Helper()
	secret := os.Getenv("SUPABASE_JWT_SECRET")
	if secret == "" {
		secret = "test-jwt-secret-for-ci-only"
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
		"amr": []map[string]any{{"method": "oauth", "timestamp": authTime.Unix()}},
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
