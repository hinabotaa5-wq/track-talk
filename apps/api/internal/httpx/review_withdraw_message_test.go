package httpx

import (
	"testing"

	"github.com/google/uuid"
)

// 返却の知らせは表題1行だけ。本文は空で、開く先は依頼の画面（本体 #609）。
func TestReviewWithdrawalPushText(t *testing.T) {
	id := uuid.New()
	got := reviewWithdrawalPush(id)
	if got.Title != "コーチが依頼を返却しました" || got.Body != "" || got.URL != "/coaching/requests/"+id.String() || got.Tag != "review-"+id.String() {
		t.Fatalf("withdrawal message: %+v", got)
	}
}
