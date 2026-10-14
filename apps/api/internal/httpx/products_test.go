package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestProductReviewFlow(t *testing.T) {
	h := setup(t)
	adminID := uuid.New()
	admin := token(t, adminID)
	user := token(t, uuid.New())
	other := token(t, uuid.New())
	if rec := doJSON(t, h, http.MethodGet, "/v1/me", admin, nil); rec.Code != http.StatusOK {
		t.Fatalf("admin profile %d", rec.Code)
	}
	setAdmin(t, adminID)

	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/products", user, map[string]string{"kind": "shoe", "brand": "B", "name": "N"}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin create %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/products", admin, map[string]string{"kind": "boot", "brand": "B", "name": "N"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad kind %d", rec.Code)
	}
	name := "テスト用スパイク " + uuid.NewString()
	rec := doJSON(t, h, http.MethodPost, "/v1/admin/products", admin, map[string]string{"kind": "spike", "brand": "Test", "name": name})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var product struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &product)
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/products", admin, map[string]string{"kind": "spike", "brand": "test", "name": name}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodGet, "/v1/products?kind=spike&q=Test", "", nil)
	var list struct {
		Products []struct {
			ID          string `json:"id"`
			ReviewCount int    `json:"review_count"`
		} `json:"products"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if rec.Code != http.StatusOK || len(list.Products) != 1 || list.Products[0].ID != product.ID {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}

	path := "/v1/products/" + product.ID + "/reviews"
	if rec := doJSON(t, h, http.MethodPost, path, "", map[string]any{"rating": 5, "body": "良い"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous post %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, path, user, map[string]any{"rating": 0, "body": "良い"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad rating %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, path, user, map[string]any{"rating": 4, "body": "反発が強い"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("review %d %s", rec.Code, rec.Body.String())
	}
	var review struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &review)
	if rec := doJSON(t, h, http.MethodPost, path, user, map[string]any{"rating": 3, "body": "二回目"}); rec.Code != http.StatusConflict {
		t.Fatalf("second review %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodGet, path, user, nil)
	var reviews struct {
		Reviews []struct {
			ID   string `json:"id"`
			Mine bool   `json:"mine"`
		} `json:"reviews"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &reviews)
	if len(reviews.Reviews) != 1 || !reviews.Reviews[0].Mine {
		t.Fatalf("reviews %s", rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodDelete, "/v1/product-reviews/"+review.ID, other, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("other delete %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/product-reviews/"+review.ID+"/hide", admin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, path, "", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &reviews)
	if len(reviews.Reviews) != 0 {
		t.Fatalf("hidden review still listed: %s", rec.Body.String())
	}

	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/products/"+product.ID+"/hide", admin, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("hide product %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodGet, "/v1/products/"+product.ID, "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("hidden product %d", rec.Code)
	}
}
