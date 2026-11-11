package httpx_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// 開示請求への応答は、応答の内容・応答者・応答日時を記録し、請求者にも見える。
func TestAdminRespondsToDisclosureInquiry(t *testing.T) {
	h := setup(t)
	user := uuid.New()
	tu := token(t, user)
	admin := adminToken(t, h)

	rec := doJSON(t, h, http.MethodPost, "/v1/me/inquiries", tu, map[string]string{
		"category": "disclosure", "subject": "所在地の開示", "body": "特定商取引法の表記の所在地と電話番号を教えてください", "email": "buyer@example.com",
	})
	if rec.Code != 201 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// 管理者以外は応答できない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/inquiries/"+created.ID+"/response", tu, map[string]string{"response": "送ります"}); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin %d %s", rec.Code, rec.Body.String())
	}

	// 空の応答は受け付けない。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/inquiries/"+created.ID+"/response", admin, map[string]string{"response": "  "}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty %d %s", rec.Code, rec.Body.String())
	}

	// 存在しない問い合わせは 404。
	if rec := doJSON(t, h, http.MethodPost, "/v1/admin/inquiries/"+uuid.NewString()+"/response", admin, map[string]string{"response": "送ります"}); rec.Code != http.StatusNotFound {
		t.Fatalf("missing %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/admin/inquiries/"+created.ID+"/response", admin, map[string]string{"response": "所在地は東京都〇〇区、電話番号は090-0000-0000です"})
	if rec.Code != 200 {
		t.Fatalf("respond %d %s", rec.Code, rec.Body.String())
	}
	var answered struct {
		ID           string  `json:"id"`
		RespondedAt  *string `json:"responded_at"`
		ResponseBody *string `json:"response_body"`
		ResponderID  *string `json:"responder_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &answered)
	if answered.RespondedAt == nil || answered.ResponseBody == nil || *answered.ResponseBody != "所在地は東京都〇〇区、電話番号は090-0000-0000です" || answered.ResponderID == nil {
		t.Fatalf("record %+v", answered)
	}

	// 管理者一覧と本人の一覧のどちらにも応答が見える。
	var list struct {
		Inquiries []struct {
			ID           string  `json:"id"`
			RespondedAt  *string `json:"responded_at"`
			ResponseBody *string `json:"response_body"`
		} `json:"inquiries"`
	}
	_ = json.Unmarshal(doJSON(t, h, http.MethodGet, "/v1/admin/inquiries", admin, nil).Body.Bytes(), &list)
	found := false
	for _, in := range list.Inquiries {
		if in.ID == created.ID {
			found = in.RespondedAt != nil && in.ResponseBody != nil && strings.Contains(*in.ResponseBody, "東京都")
		}
	}
	if !found {
		t.Fatalf("admin list %+v", list)
	}
	_ = json.Unmarshal(doJSON(t, h, http.MethodGet, "/v1/me/inquiries", tu, nil).Body.Bytes(), &list)
	found = false
	for _, in := range list.Inquiries {
		if in.ID == created.ID {
			found = in.RespondedAt != nil && in.ResponseBody != nil && strings.Contains(*in.ResponseBody, "東京都")
		}
	}
	if !found {
		t.Fatalf("mine %+v", list)
	}

	// 再応答は上書きする（訂正のため）。
	rec = doJSON(t, h, http.MethodPost, "/v1/admin/inquiries/"+created.ID+"/response", admin, map[string]string{"response": "訂正します"})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "訂正します") {
		t.Fatalf("re-respond %d %s", rec.Code, rec.Body.String())
	}
}
