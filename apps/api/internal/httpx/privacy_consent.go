package httpx

import (
	"errors"
	"io"
	"net/http"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 海外の委託先(米国の事業者)に個人情報を預けることへの同意(個人情報保護法28条)。
// 説明の中身を変えたら版を上げ、全員にもう一度同意してもらう。
const (
	overseasConsentKind    = "overseas"
	overseasConsentVersion = "overseas-2026-10"
)

func (s *Server) hasOverseasConsent(r *http.Request, p db.Profile) (bool, error) {
	return s.q.HasPrivacyConsent(r.Context(), db.HasPrivacyConsentParams{
		UserID:  p.ID,
		Kind:    overseasConsentKind,
		Version: overseasConsentVersion,
	})
}

// 本文は省略できる。登録画面で同意した版を version で渡されたときは、いまの版と同じときだけ記録する
// (登録のあとで説明を変えていたら、新しい版の説明で同意してもらう)。
func (s *Server) consentOverseas(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := DecodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if body.Version != "" && body.Version != overseasConsentVersion {
		Error(w, http.StatusConflict, "consent_version_changed", "説明の内容が更新されています。もう一度確認して同意してください")
		return
	}
	if err := s.q.RecordPrivacyConsent(r.Context(), db.RecordPrivacyConsentParams{
		UserID:  p.ID,
		Kind:    overseasConsentKind,
		Version: overseasConsentVersion,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
