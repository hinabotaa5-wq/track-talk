package httpx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func seedArchive(t *testing.T, place string) string {
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
	var id string
	err = pool.QueryRow(ctx, `INSERT INTO injury_archives (body_part, injured_month, archived_month, stage_timeline, place, situation, posts)
		VALUES ('knee', '2026-09-01', '2026-10-01', '[{"stage":"resting","days":0}]', $1, '山田コーチに言われた', '[{"title":"山田さんへ","body":"山田コーチありがとう","updates":[{"stage":"rehab","body":"山田さんのおかげ"}]}]')
		RETURNING id::text`, place).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

type archiveListResp struct {
	Archives []struct {
		ID           string `json:"id"`
		BodyPart     string `json:"body_part"`
		InjuredMonth string `json:"injured_month"`
		Place        string `json:"place"`
		Situation    string `json:"situation"`
		ReviewStatus string `json:"review_status"`
		Posts        []struct {
			Title   string `json:"title"`
			Body    string `json:"body"`
			Updates []struct {
				Body string `json:"body"`
			} `json:"updates"`
		} `json:"posts"`
	} `json:"archives"`
	Meta struct {
		Total int64 `json:"total"`
	} `json:"meta"`
}

func TestAdminReviewsInjuryArchives(t *testing.T) {
	h := setup(t)
	clearArchives(t)
	id := seedArchive(t, "山田公園")

	adminID := uuid.New()
	admin := token(t, adminID)
	doJSON(t, h, http.MethodGet, "/v1/me", admin, nil)
	setAdmin(t, adminID)
	member := token(t, uuid.New())

	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/injury-archives", member, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin list %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/admin/injury-archives?status=other", admin, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad status %d", rec.Code)
	}

	list := func(status string) archiveListResp {
		rec := doJSON(t, h, http.MethodGet, "/v1/admin/injury-archives?status="+status, admin, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("list %d %s", rec.Code, rec.Body.String())
		}
		var out archiveListResp
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := list("pending"); got.Meta.Total != 1 || got.Archives[0].InjuredMonth != "2026-09" || len(got.Archives[0].Posts) != 1 {
		t.Fatalf("pending list %+v", got)
	}

	// 伏せ字にすると、場所・状況・投稿・経過のどこでも置き換わり、もう一度確かめるために pending に戻る。
	if rec := doJSON(t, h, http.MethodPatch, "/v1/admin/injury-archives/"+id, admin, map[string]any{"review_status": "approved"}); rec.Code != http.StatusNoContent {
		t.Fatalf("approve %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/injury-archives/"+id+"/mask", admin, map[string]any{"words": []string{}}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty words %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/injury-archives/"+id+"/mask", admin, map[string]any{"words": []string{"山田", "山田コーチ"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("mask %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "山田") {
		t.Fatalf("name left after masking: %s", rec.Body.String())
	}
	got := list("pending")
	if got.Meta.Total != 1 || got.Archives[0].ReviewStatus != "pending" || !strings.Contains(got.Archives[0].Place, "[伏せ]") ||
		got.Archives[0].Posts[0].Body != "[伏せ]ありがとう" || got.Archives[0].Posts[0].Updates[0].Body != "[伏せ]さんのおかげ" {
		t.Fatalf("after mask %+v", got)
	}

	if rec := doJSON(t, h, http.MethodPatch, "/v1/admin/injury-archives/"+id, admin, map[string]any{"review_status": "weird"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad review status %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/admin/injury-archives/"+id, admin, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("delete before rejecting %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/admin/injury-archives/"+id, admin, map[string]any{"review_status": "approved"}); rec.Code != http.StatusNoContent {
		t.Fatalf("approve %d", rec.Code)
	}
	if list("approved").Meta.Total != 1 || list("pending").Meta.Total != 0 {
		t.Fatalf("approved list")
	}
	if rec := doJSON(t, h, http.MethodPatch, "/v1/admin/injury-archives/"+id, admin, map[string]any{"review_status": "rejected"}); rec.Code != http.StatusNoContent {
		t.Fatalf("reject %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/admin/injury-archives/"+id, admin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete rejected %d", rec.Code)
	}
	if list("").Meta.Total != 0 {
		t.Fatalf("archive remains after delete")
	}
}
