package httpx

import (
	"errors"
	"net/http"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
)

func (s *Server) signVideoUpload(w http.ResponseWriter, r *http.Request) {
	s.signUpload(w, r, "video", "動画")
}

func (s *Server) signImageUpload(w http.ResponseWriter, r *http.Request) {
	s.signUpload(w, r, "image", "写真")
}

func (s *Server) signUpload(w http.ResponseWriter, r *http.Request, resource, noun string) {
	if _, ok := s.requireProfile(w, r); !ok {
		return
	}
	signed, err := s.cloud.SignedUpload(time.Now(), resource)
	if err != nil {
		writeAssetReadyError(w, err, noun)
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
