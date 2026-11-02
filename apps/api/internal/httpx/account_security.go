package httpx

import (
	"net/http"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// staffMFAMissing は、管理者の操作に二段階認証が要るのに、いまのログインで済ませていないときに true。
// コーチには求めない(外部のコーチに認証アプリの登録を強いると、始めるときの負担になるため。2026-11-02 運営者決定)。
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

// staffMFARequired は、この人が管理者で、いまのログインに二段階認証が足りないかを返す。
// /v1/me で返し、画面は管理の画面を開く前に確認を求める。
func (s *Server) staffMFARequired(r *http.Request, p db.Profile) (bool, error) {
	return p.IsAdmin && s.staffMFAMissing(r), nil
}
