package httpx

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

func (s *Server) signVideoUpload(w http.ResponseWriter, r *http.Request) {
	s.signUpload(w, r, "video", "動画")
}

func (s *Server) signImageUpload(w http.ResponseWriter, r *http.Request) {
	s.signUpload(w, r, "image", "写真")
}

func (s *Server) signUpload(w http.ResponseWriter, r *http.Request, resource, noun string) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if !s.uploadQuotaLeft(w, r, p.ID) {
		return
	}
	signed, err := s.cloud.SignedUpload(time.Now(), resource)
	if err != nil {
		writeAssetReadyError(w, err, noun)
		return
	}
	if err := s.q.RecordUpload(r.Context(), db.RecordUploadParams{PublicID: signed.PublicID, OwnerID: p.ID}); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, signed)
}

func writeVideoReadyError(w http.ResponseWriter, err error) {
	writeAssetReadyError(w, err, "動画")
}

func writeAssetReadyError(w http.ResponseWriter, err error, noun string) {
	switch {
	case errors.Is(err, cloudinary.ErrNotConfigured):
		Error(w, http.StatusServiceUnavailable, "unavailable", noun+"のアップロードはまだ設定されていません")
	case errors.Is(err, cloudinary.ErrInvalid):
		Error(w, http.StatusUnprocessableEntity, "validation_error", noun+"IDが不正です")
	case errors.Is(err, cloudinary.ErrNotFound):
		Error(w, http.StatusUnprocessableEntity, "validation_error", noun+"が見つかりません")
	case errors.Is(err, cloudinary.ErrRejected):
		Error(w, http.StatusUnprocessableEntity, "validation_error", "対応していないファイル形式です")
	case errors.Is(err, cloudinary.ErrTooLarge):
		Error(w, http.StatusUnprocessableEntity, "validation_error", noun+"はサイズの上限を超えています")
	case errors.Is(err, cloudinary.ErrTooLong):
		Error(w, http.StatusUnprocessableEntity, "validation_error", noun+"は30秒以内にしてください")
	default:
		Error(w, http.StatusBadGateway, "unavailable", noun+"の確認に失敗しました")
	}
}

// maxUploadSignsPerDay は、1人が1日(24時間)に受け取れる署名の数。署名があれば Cloudinary に直接アップロードできるので、数を絞る。
const maxUploadSignsPerDay = 30

// uploadQuotaLeft は、署名の数が1日の上限に届いていないかを確かめる。届いていれば 429 を書いて false を返す。
func (s *Server) uploadQuotaLeft(w http.ResponseWriter, r *http.Request, owner uuid.UUID) bool {
	n, err := s.q.CountRecentUploads(r.Context(), owner)
	if err != nil {
		MapDBError(w, err)
		return false
	}
	if n >= maxUploadSignsPerDay {
		Error(w, http.StatusTooManyRequests, "rate_limited", "アップロードの回数が1日の上限に達しました。明日またお試しください")
		return false
	}
	return true
}

// uploadIssuedToMe は、この public_id の署名を、ログイン中の本人に出したかを確かめる。
// public_id は公開 URL や署名付き URL に出るので、ID を知っているだけでは他人の画像・動画を使えないようにする。
// 違えば 422 を書いて false を返す。
func (s *Server) uploadIssuedToMe(w http.ResponseWriter, r *http.Request, owner uuid.UUID, publicID, field, noun string) bool {
	ok, err := s.q.UploadIssuedTo(r.Context(), db.UploadIssuedToParams{PublicID: publicID, OwnerID: owner})
	if err != nil {
		MapDBError(w, err)
		return false
	}
	if !ok {
		FieldError(w, map[string]string{field: noun + "を選び直してください"})
		return false
	}
	return true
}
