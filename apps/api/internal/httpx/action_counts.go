package httpx

import "net/http"

// coachingActionCounts は、コーチタブの円ボタン（選手・コーチ）に出す「自分が対応しないと進まない用事」の件数を返す（本体 #891）。
// 通知ベルの未読数とは別系統。本人の ID だけで数え、コーチの行が無い利用者はコーチ側が 0 になる（エラーにしない）。
func (s *Server) coachingActionCounts(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	player, err := s.q.CountPlayerActionItems(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	coach, err := s.q.CountCoachActionItems(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 選手ページのタイル別の件数（本体 #1003）。player_count の (a)＋(b) をタイルごとに分けたもので、
	// どのタイルにも当たらない未読（例：/me/coaching ちょうど）の分だけタイル合計は player_count より少ない。
	tiles, err := s.q.CountPlayerTileItems(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"player_count":          player,
		"coach_count":           coach,
		"player_one_time_count": tiles.PlayerOneTimeCount,
		"player_personal_count": tiles.PlayerPersonalCount,
		"player_support_count":  tiles.PlayerSupportCount,
		"player_ticket_count":   tiles.PlayerTicketCount,
	})
}
