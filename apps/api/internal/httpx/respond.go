package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type errorBody struct {
	Error errDetail `json:"error"`
}

type errDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, errorBody{Error: errDetail{Code: code, Message: message}})
}

func FieldError(w http.ResponseWriter, fields map[string]string) {
	JSON(w, http.StatusUnprocessableEntity, errorBody{Error: errDetail{
		Code:    "validation_error",
		Message: "入力内容を確認してください",
		Fields:  fields,
	}})
}

func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func MapDBError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			msg := "すでに登録されています"
			if pgErr.ConstraintName == "posts_youtube_video_id_active_idx" {
				msg = "この動画のスレはすでにあります"
			}
			Error(w, http.StatusConflict, "conflict", msg)
			return true
		}
		if pgErr.Code == "23503" {
			Error(w, http.StatusUnprocessableEntity, "validation_error", "関連データが不正です")
			return true
		}
	}
	slog.Error("db error", "err", err)
	Error(w, http.StatusInternalServerError, "internal_error", "サーバーエラーが発生しました")
	return true
}
