package httpx

import (
	"net/http"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// Update both API and Web versions whenever a document is revised, to require fresh consent.
// terms-2026-11-25b: 本体 #1031（チケット販売終了・紹介特典クーポン）。同日の terms-2026-11-25 が公開済みのため、枝番で改める。
// terms-2026-11-26・privacy-2026-11-26: 本体 #1045（生まれた年の取得、購入前チェックを未成年だけに表示）。
const (
	termsDocumentVersion      = "terms-2026-11-26"
	guidelinesDocumentVersion = "guidelines-2026-11-25"
	privacyDocumentVersion    = "privacy-2026-11-26"
	// Provisional retention approved in #953; replace here after legal confirmation.
	documentConsentRetentionYears = 5
)

func currentDocumentVersions() map[string]string {
	return map[string]string{"terms": termsDocumentVersion, "guidelines": guidelinesDocumentVersion, "privacy": privacyDocumentVersion}
}

func (s *Server) hasDocumentConsents(r *http.Request, p db.Profile) (bool, error) {
	return s.q.HasDocumentConsents(r.Context(), db.HasDocumentConsentsParams{
		UserID: p.ID, TermsVersion: termsDocumentVersion, GuidelinesVersion: guidelinesDocumentVersion, PrivacyVersion: privacyDocumentVersion,
	})
}

func (s *Server) consentDocuments(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var body struct {
		Versions map[string]string `json:"versions"`
	}
	if err := DecodeJSON(r, &body); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	for document, version := range currentDocumentVersions() {
		if body.Versions[document] != version {
			Error(w, http.StatusConflict, "consent_version_changed", "説明の内容が更新されています。もう一度確認して同意してください")
			return
		}
	}
	if err := s.q.RecordDocumentConsents(r.Context(), db.RecordDocumentConsentsParams{
		UserID: p.ID, TermsVersion: termsDocumentVersion, GuidelinesVersion: guidelinesDocumentVersion, PrivacyVersion: privacyDocumentVersion,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
