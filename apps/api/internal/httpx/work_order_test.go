package httpx_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type workOrderTermsView struct {
	Items []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	} `json:"items"`
	AmountYen  int    `json:"amount_yen"`
	DeliverBy  string `json:"deliver_by"`
	PaymentDue string `json:"payment_due"`
}

// フォーム添削を打診するときは、引き受ける前に取引条件(9項目)を見せ、引き受けたら書き留める。
func TestReviewOfferShowsAndRecordsWorkOrderTerms(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripeConfig(t, nil)
	admin := adminToken(t, h)
	_, coach := onDutyCoach(t, h, admin)
	_, owner := demoRequester(t, h, admin)
	buyFiveTickets(t, h, owner)
	request := ticketReview(t, h, owner)

	rec := doJSON(t, h, http.MethodGet, "/v1/coach/requests", coach, nil)
	var list struct {
		Offers []struct {
			ID    string             `json:"id"`
			Terms workOrderTermsView `json:"terms"`
		} `json:"offers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Offers) != 1 {
		t.Fatalf("offers %d %s", rec.Code, rec.Body.String())
	}
	offer := list.Offers[0]
	// チケット2枚(割引前1,200円)の70%。
	if offer.Terms.AmountYen != 840 || len(offer.Terms.Items) != 9 || offer.Terms.PaymentDue == "" || offer.Terms.DeliverBy == "" {
		t.Fatalf("terms before accept: %+v", offer.Terms)
	}

	rec = doJSON(t, h, http.MethodPost, "/v1/coach/offers/"+offer.ID+"/accept", coach, nil)
	var accepted struct {
		WorkOrder struct {
			TermsVersion int                `json:"terms_version"`
			Terms        workOrderTermsView `json:"terms"`
		} `json:"work_order"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil || rec.Code != 200 {
		t.Fatalf("accept %d %s", rec.Code, rec.Body.String())
	}
	if accepted.WorkOrder.TermsVersion != 1 || accepted.WorkOrder.Terms.AmountYen != 840 || len(accepted.WorkOrder.Terms.Items) != 9 {
		t.Fatalf("work order after accept: %s", rec.Body.String())
	}

	// 後から依頼の画面で見返せる。
	rec = doJSON(t, h, http.MethodGet, "/v1/coach/requests/"+request, coach, nil)
	var detail struct {
		WorkOrder *struct {
			Terms workOrderTermsView `json:"terms"`
		} `json:"work_order"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil || detail.WorkOrder == nil || detail.WorkOrder.Terms.AmountYen != 840 {
		t.Fatalf("detail %d %s", rec.Code, rec.Body.String())
	}
}

// フォーム添削の追加相談も、引き受ける前に取引条件を見せ、引き受けたら書き留める。
func TestReviewConsultationShowsAndRecordsWorkOrderTerms(t *testing.T) {
	fakeCoachingCloudinary(t)
	h, _ := setupStripe(t)
	owner, coach, root := answeredTicketRoom(t, h)
	path := "/v1/coaching/requests/" + root + "/consultations"
	c := consultationDraft(t, h, owner, path, uuid.NewString(), "次の大会ではどこを意識しますか")
	consultationAction(t, h, owner, c.ID, "submit", nil)

	room := func() map[string]json.RawMessage {
		rec := doJSON(t, h, http.MethodGet, path, coach, nil)
		var out struct {
			Consultations []map[string]json.RawMessage `json:"consultations"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Consultations) != 1 {
			t.Fatalf("room %d %s", rec.Code, rec.Body.String())
		}
		return out.Consultations[0]
	}
	var terms workOrderTermsView
	if err := json.Unmarshal(room()["terms"], &terms); err != nil || terms.AmountYen != 420 || len(terms.Items) != 9 {
		t.Fatalf("terms before accept: %+v %v", terms, err)
	}
	consultationAction(t, h, coach, c.ID, "accept", nil)
	var wo struct {
		Terms workOrderTermsView `json:"terms"`
	}
	if err := json.Unmarshal(room()["work_order"], &wo); err != nil || wo.Terms.AmountYen != 420 {
		t.Fatalf("work order after accept: %+v %v", wo, err)
	}
	var count int
	queryRowArgs(t, `SELECT count(*) FROM coach_work_orders WHERE consultation_id=$1`, []any{c.ID}, &count)
	if count != 1 {
		t.Fatalf("work orders %d", count)
	}
}
