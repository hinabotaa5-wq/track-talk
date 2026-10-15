package httpx

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgconn"
)

func validProductKind(kind string) bool {
	return kind == "shoe" || kind == "spike"
}

func productJSON(id uuid.UUID, kind, brand, name, imageURL, productURL string, discontinued bool, reviewCount int64, avg float64) map[string]any {
	return map[string]any{
		"id":           id,
		"kind":         kind,
		"brand":        brand,
		"name":         name,
		"image_url":    imageURL,
		"product_url":  productURL,
		"discontinued": discontinued,
		"review_count": reviewCount,
		"avg_rating":   avg,
	}
}

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "shoe"
	}
	if !validProductKind(kind) {
		FieldError(w, map[string]string{"kind": "シューズかスパイクを選んでください"})
		return
	}
	q := likeContains(r.URL.Query().Get("q"))
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountProducts(r.Context(), db.CountProductsParams{Kind: kind, Q: q})
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListProducts(r.Context(), db.ListProductsParams{Kind: kind, Q: q, RowLimit: per, RowOffset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, productJSON(row.ID, row.Kind, row.Brand, row.Name, row.ImageUrl, row.ProductUrl, row.Discontinued, row.ReviewCount, row.AvgRating))
	}
	JSON(w, http.StatusOK, map[string]any{"products": out, "meta": meta{Page: int(page), PerPage: int(per), Total: total}})
}

func (s *Server) getProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	row, err := s.q.GetProduct(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, productJSON(row.ID, row.Kind, row.Brand, row.Name, row.ImageUrl, row.ProductUrl, row.Discontinued, row.ReviewCount, row.AvgRating))
}

func (s *Server) listProductReviews(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.q.GetProduct(r.Context(), id); err != nil {
		MapDBError(w, err)
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountProductReviews(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListProductReviews(r.Context(), db.ListProductReviewsParams{ProductID: id, Limit: per, Offset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	uid, authed := UserID(r.Context())
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":         row.ID,
			"product_id": row.ProductID,
			"rating":     row.Rating,
			"body":       row.Body,
			"mine":       authed && uid == row.AuthorID,
			"created_at": row.CreatedAt,
		})
	}
	JSON(w, http.StatusOK, map[string]any{"reviews": out, "meta": meta{Page: int(page), PerPage: int(per), Total: total}})
}

type productReviewReq struct {
	Rating int    `json:"rating"`
	Body   string `json:"body"`
}

func (s *Server) createProductReview(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	productID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req productReviewReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	if req.Rating < 1 || req.Rating > 5 {
		fields["rating"] = "評価を選んでください"
	}
	if e := Body(req.Body, 2000); e != "" {
		fields["body"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	if _, err := s.q.GetProduct(r.Context(), productID); err != nil {
		MapDBError(w, err)
		return
	}
	row, err := s.q.CreateProductReview(r.Context(), db.CreateProductReviewParams{
		ID:        uuid.New(),
		ProductID: productID,
		AuthorID:  p.ID,
		Rating:    int16(req.Rating),
		Body:      strings.TrimSpace(req.Body),
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "この商品にはすでにレビューを書いています")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{
		"id":         row.ID,
		"product_id": row.ProductID,
		"rating":     row.Rating,
		"body":       row.Body,
		"mine":       true,
		"created_at": row.CreatedAt,
	})
}

func (s *Server) deleteProductReview(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.SoftDeleteProductReview(r.Context(), db.SoftDeleteProductReviewParams{ID: id, AuthorID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusForbidden, "forbidden", "削除できません")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createProductReq struct {
	Kind       string `json:"kind"`
	Brand      string `json:"brand"`
	Name       string `json:"name"`
	ImageURL   string `json:"image_url"`
	ProductURL string `json:"product_url"`
}

func (s *Server) createProduct(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req createProductReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.Brand = strings.TrimSpace(req.Brand)
	req.Name = strings.TrimSpace(req.Name)
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	req.ProductURL = strings.TrimSpace(req.ProductURL)
	fields := map[string]string{}
	if !validProductKind(req.Kind) {
		fields["kind"] = "シューズかスパイクを選んでください"
	}
	if n := utf8.RuneCountInString(req.Brand); n < 1 || n > 60 {
		fields["brand"] = "ブランドは60文字以内で入力してください"
	}
	if n := utf8.RuneCountInString(req.Name); n < 1 || n > 120 {
		fields["name"] = "商品名は120文字以内で入力してください"
	}
	if req.ImageURL != "" && (!strings.HasPrefix(req.ImageURL, "https://") || len(req.ImageURL) > 500) {
		fields["image_url"] = "画像URLはhttpsで始まる500文字以内にしてください"
	}
	if e := productURLError(req.ProductURL); e != "" {
		fields["product_url"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	id, err := s.q.CreateProduct(r.Context(), db.CreateProductParams{
		ID:         uuid.New(),
		Kind:       req.Kind,
		Brand:      req.Brand,
		Name:       req.Name,
		ImageUrl:   req.ImageURL,
		ProductUrl: req.ProductURL,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		Error(w, http.StatusConflict, "conflict", "同じ商品がすでに登録されています")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, productJSON(id, req.Kind, req.Brand, req.Name, req.ImageURL, req.ProductURL, false, 0, 0))
}

// 公式ページのURL。空は「リンクなし」。外部に飛ばすので https のみ許す。
func productURLError(v string) string {
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "https://") || len(v) > 500 || strings.ContainsAny(v, " \t\r\n") {
		return "公式ページのURLはhttpsで始まる500文字以内にしてください"
	}
	return ""
}

type updateProductReq struct {
	ProductURL   string `json:"product_url"`
	Discontinued bool   `json:"discontinued"`
}

func (s *Server) updateProduct(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req updateProductReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.ProductURL = strings.TrimSpace(req.ProductURL)
	if e := productURLError(req.ProductURL); e != "" {
		FieldError(w, map[string]string{"product_url": e})
		return
	}
	n, err := s.q.UpdateProduct(r.Context(), db.UpdateProductParams{ID: id, ProductUrl: req.ProductURL, Discontinued: req.Discontinued})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type productRequestReq struct {
	Kind  string `json:"kind"`
	Brand string `json:"brand"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

// 「この商品がない」リクエスト。問い合わせ(source = product_request)として管理画面に出す。
func (s *Server) createProductRequest(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req productRequestReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	req.Brand = strings.TrimSpace(req.Brand)
	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)
	fields := map[string]string{}
	if !validProductKind(req.Kind) {
		fields["kind"] = "シューズかスパイクを選んでください"
	}
	if n := utf8.RuneCountInString(req.Brand); n < 1 || n > 60 {
		fields["brand"] = "ブランドは60文字以内で入力してください"
	}
	if n := utf8.RuneCountInString(req.Name); n < 1 || n > 120 {
		fields["name"] = "商品名は120文字以内で入力してください"
	}
	if e := productURLError(req.URL); e != "" {
		fields["url"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	subject := req.Brand + " " + req.Name
	if utf8.RuneCountInString(subject) > 80 {
		subject = string([]rune(subject)[:80])
	}
	body := "種類: " + req.Kind + "\nブランド: " + req.Brand + "\n商品名: " + req.Name
	if req.URL != "" {
		body += "\n公式ページ: " + req.URL
	}
	if _, err := s.q.CreateInquiry(r.Context(), db.CreateInquiryParams{
		ID:       uuid.New(),
		UserID:   p.ID,
		Category: "request",
		Subject:  subject,
		Body:     body,
		Source:   "product_request",
	}); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) hideProduct(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.HideProduct(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) hideProductReview(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.HideProductReview(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
