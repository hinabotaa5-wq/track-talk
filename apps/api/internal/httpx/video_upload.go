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
	s.issueUpload(w, r, p.ID, noun, func() (cloudinary.UploadSignature, error) {
		return s.cloud.SignedUpload(time.Now(), resource)
	})
}

// issueUpload は、1日の上限を確かめてから署名を出し、出した相手を記録する。
// 同時に頼まれても上限を超えないよう、確認から記録までを本人ごとのロックの中で行う。
func (s *Server) issueUpload(w http.ResponseWriter, r *http.Request, owner uuid.UUID, noun string, sign func() (cloudinary.UploadSignature, error)) {
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if err := q.LockUploadOwner(r.Context(), owner); err != nil {
		MapDBError(w, err)
		return
	}
	n, err := q.CountRecentUploads(r.Context(), owner)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n >= maxUploadSignsPerDay {
		Error(w, http.StatusTooManyRequests, "rate_limited", "アップロードの回数が1日の上限に達しました。明日またお試しください")
		return
	}
	signed, err := sign()
	if err != nil {
		writeAssetReadyError(w, err, noun)
		return
	}
	if err := q.RecordUpload(r.Context(), db.RecordUploadParams{PublicID: signed.PublicID, OwnerID: owner}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
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
