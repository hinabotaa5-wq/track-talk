package httpx

import "testing"

func TestLooksLikeContact(t *testing.T) {
	blocked := []string{
		"詳しくは https://example.com を見てください",
		"www.example.jp",
		"youtube.com で探してみて",
		"coach@example.com に送って",
		"インスタの @track_coach をフォローして",
		"090-1234-5678 に電話して",
		"０９０１２３４５６７８",
		"LINE で送ってください",
		"ｌｉｎｅ交換しましょう",
		"DMください",
		"IDは tt123 です",
		"インスタで動画を見せて",
	}
	for _, s := range blocked {
		if !looksLikeContact(s) {
			t.Errorf("should block: %q", s)
		}
	}
	allowed := []string{
		"スタートラインに手をつく位置を、指1本ぶん前にしましょう",
		"1台目までを8歩で行けています。110mハードルは13秒台を目指せます",
		"100m 10秒95、200m 21秒80",
		"ラインを意識して、真っすぐ走りましょう",
		"腕振りは肘を90度に保つ",
		"ミニハードル5台を1.5m間隔で並べて、3セット",
	}
	for _, s := range allowed {
		if looksLikeContact(s) {
			t.Errorf("should allow: %q", s)
		}
	}
}
