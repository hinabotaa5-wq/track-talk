package httpx

import (
	"errors"
	"net/http"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// staffMFAMissing は、管理者・コーチの操作に二段階認証が要るのに、いまのログインで済ませていないときに true。
func (s *Server) staffMFAMissing(r *http.Request) bool {
	if !s.cfg.StaffMFARequired {
		return false
	}
	identity, ok := userIdentity(r.Context())
	return !ok || identity.AAL != "aal2"
}

// requireStaffMFA は、二段階認証が済んでいなければ 403(mfa_required)を返して false。
// 画面はこのコードを見て、二段階認証の確認・設定の画面を出す。
func (s *Server) requireStaffMFA(w http.ResponseWriter, r *http.Request) bool {
	if s.staffMFAMissing(r) {
		Error(w, http.StatusForbidden, "mfa_required", "この操作には二段階認証が必要です。マイページの「二段階認証」で設定・確認してください")
		return false
	}
	return true
}

// staffMFARequired は、この人が管理者か承認済みコーチで、いまのログインに二段階認証が足りないかを返す。
// /v1/me で返し、画面は管理・コーチの画面を開く前に確認を求める。
func (s *Server) staffMFARequired(r *http.Request, p db.Profile) (bool, error) {
	if !s.staffMFAMissing(r) {
		return false, nil
	}
	if p.IsAdmin {
		return true, nil
	}
	c, err := s.q.GetCoachProfile(r.Context(), p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return c.Status == "approved", nil
}
