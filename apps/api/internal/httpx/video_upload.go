package httpx

import (
	"errors"
	"net/http"
	"time"

	"github.com/hinata/track-talk/apps/api/internal/cloudinary"
)

func (s *Server) signVideoUpload(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireProfile(w, r); !ok {
		return
	}
	signed, err := s.cloud.SignedUpload(time.Now())
	if err != nil {
		writeVideoReadyError(w, err)
		return
	}
	JSON(w, http.StatusOK, signed)
}

func writeVideoReadyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, cloudinary.ErrNotConfigured):
		Error(w, http.StatusServiceUnavailable, "unavailable", "動画のアップロードはまだ設定されていません")
	case errors.Is(err, cloudinary.ErrInvalid):
		Error(w, http.StatusUnprocessableEntity, "validation_error", "動画IDが不正です")
	case errors.Is(err, cloudinary.ErrNotFound):
		Error(w, http.StatusUnprocessableEntity, "validation_error", "動画が見つかりません")
	default:
		Error(w, http.StatusBadGateway, "unavailable", "動画の確認に失敗しました")
	}
}
