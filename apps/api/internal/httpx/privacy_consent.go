package httpx

import (
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

func (s *Server) consentOverseas(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
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
