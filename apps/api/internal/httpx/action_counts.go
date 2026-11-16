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
	JSON(w, http.StatusOK, map[string]any{
		"player_count": player,
		"coach_count":  coach,
	})
}
