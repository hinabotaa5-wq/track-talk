package httpx

import "testing"

func TestNormalizeRatingNoShow(t *testing.T) {
	score, _, fields := normalizeRating(ratingReq{Score: "no_show"})
	if score != "no_show" || len(fields) != 0 {
		t.Fatalf("no_show が通らない: score=%q fields=%v", score, fields)
	}
	if _, _, fields := normalizeRating(ratingReq{Score: "unknown"}); fields["score"] == "" {
		t.Fatal("不明な評価がエラーにならない")
	}
}
