package httpx

import "net/http"

// 日次照合用。異常と通常の処理待ちを区別し、会話本文や決済の秘密情報は返さない。
func (s *Server) adminReconciliation(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	result, err := s.q.CoachingReconciliation(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, result)
}
