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
	if wantsEager(r) {
		p, ok := s.requireProfile(w, r)
		if !ok {
			return
		}
		s.issueUpload(w, r, p.ID, "動画", func() (cloudinary.UploadSignature, error) {
			return s.cloud.SignedAsyncVideoUpload(time.Now())
		})
		return
	}
	s.signUpload(w, r, "video", "動画")
}

// wantsEager は、画面が eager と eager_async を Cloudinary に送れる（?eager=1 を付けた）かを返す。
// 付けない古い画面には、これまでどおり保存前に縮める署名を出す。eager を含む署名を送らない画面に出すと、
// 署名が合わずにアップロードが断られるため。
func wantsEager(r *http.Request) bool {
	return r.URL.Query().Get("eager") == "1"
}

// videoUploadStatus は、上げ終えた動画が投稿・送信に使える状態かを返す（POST /v1/uploads/video/status）。
// 大きな動画は、配信用の変換を Cloudinary が裏で作り終えるまで ready=false になる。画面はこれを待ってから送る。
func (s *Server) videoUploadStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		PublicID string `json:"public_id"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "リクエストを読めませんでした")
		return
	}
	var check func() error
	switch {
	case cloudinary.ValidPublicID(req.PublicID):
		check = func() error { return s.cloud.Ready(r.Context(), req.PublicID) }
	case cloudinary.ValidCoachingPublicID(req.PublicID):
		check = func() error { return s.cloud.CoachingVideoReady(r.Context(), req.PublicID) }
	default:
		FieldError(w, map[string]string{"public_id": "動画を選び直してください"})
		return
	}
	if !s.uploadIssuedToMe(w, r, p.ID, req.PublicID, "public_id", "動画") {
		return
	}
	err := check()
	if errors.Is(err, cloudinary.ErrProcessing) {
		JSON(w, http.StatusOK, map[string]bool{"ready": false})
		return
	}
	if err != nil {
		writeVideoReadyError(w, err)
		return
	}
	// 使える状態になったら、縮めずに保存した元ファイルを縮めた版へ置き換える（対象でなければ何もしない）。
	s.shrinkOriginalSoon(req.PublicID)
	JSON(w, http.StatusOK, map[string]bool{"ready": true})
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
	case errors.Is(err, cloudinary.ErrProcessing):
		Error(w, http.StatusConflict, "processing", noun+"を処理しています。少し待ってからもう一度送ってください")
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
