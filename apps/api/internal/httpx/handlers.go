package httpx

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

func (s *Server) queryPage(r *http.Request) (page, per, offset int32) {
	pageN, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perN, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	p, pp := ClampPage(pageN, perN)
	return int32(p), int32(pp), int32((p - 1) * pp)
}

func textPtr(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) requireProfile(w http.ResponseWriter, r *http.Request) (db.Profile, bool) {
	uid, ok := UserID(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
		return db.Profile{}, false
	}
	p, err := s.q.GetProfile(r.Context(), uid)
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusConflict, "profile_required", "プロフィールを作成してください")
		return db.Profile{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.Profile{}, false
	}
	return p, true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (db.Profile, bool) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return db.Profile{}, false
	}
	if !p.IsAdmin {
		Error(w, http.StatusForbidden, "forbidden", "権限がありません")
		return db.Profile{}, false
	}
	return p, true
}

func profileJSON(p db.Profile) map[string]any {
	return map[string]any{
		"id":         p.ID,
		"nickname":   p.Nickname,
		"bio":        p.Bio,
		"is_admin":   p.IsAdmin,
		"created_at": p.CreatedAt,
		"updated_at": p.UpdatedAt,
	}
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	JSON(w, http.StatusOK, profileJSON(p))
}

type profileReq struct {
	Nickname string `json:"nickname"`
	Bio      string `json:"bio"`
}

func (s *Server) upsertProfile(w http.ResponseWriter, r *http.Request) {
	uid, _ := UserID(r.Context())
	var req profileReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	if e := Nickname(req.Nickname); e != "" {
		fields["nickname"] = e
	}
	if e := Bio(req.Bio); e != "" {
		fields["bio"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	p, err := s.q.UpsertProfile(r.Context(), db.UpsertProfileParams{
		ID:       uid,
		Nickname: strings.TrimSpace(req.Nickname),
		Bio:      strings.TrimSpace(req.Bio),
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, profileJSON(p))
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	p, err := s.q.GetProfile(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{
		"id":         p.ID,
		"nickname":   p.Nickname,
		"bio":        p.Bio,
		"created_at": p.CreatedAt,
	})
}

func (s *Server) listUserPosts(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	s.listPostsFiltered(w, r, "new", nil, nil, &id)
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListCategories(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"categories": rows})
}

func (s *Server) listCompetitions(w http.ResponseWriter, r *http.Request) {
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountCompetitions(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListCompetitions(r.Context(), db.ListCompetitionsParams{Limit: per, Offset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"competitions": rows, "meta": meta{Page: int(page), PerPage: int(per), Total: total}})
}

func (s *Server) getCompetition(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	row, err := s.q.GetCompetition(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, row)
}

func (s *Server) listCompetitionPosts(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	s.listPostsFiltered(w, r, r.URL.Query().Get("sort"), nil, &id, nil)
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var cat *string
	if v := strings.TrimSpace(q.Get("category_id")); v != "" {
		cat = &v
	}
	var comp *uuid.UUID
	if v := strings.TrimSpace(q.Get("competition_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			Error(w, http.StatusUnprocessableEntity, "validation_error", "competition_idが不正です")
			return
		}
		comp = &id
	}
	var author *uuid.UUID
	if v := strings.TrimSpace(q.Get("author_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			Error(w, http.StatusUnprocessableEntity, "validation_error", "author_idが不正です")
			return
		}
		author = &id
	}
	s.listPostsFiltered(w, r, q.Get("sort"), cat, comp, author)
}

func (s *Server) listPostsFiltered(w http.ResponseWriter, r *http.Request, sort string, cat *string, comp, author *uuid.UUID) {
	page, per, offset := s.queryPage(r)
	popular := sort == "popular"
	count, err := s.q.CountPosts(r.Context(), db.CountPostsParams{
		CategoryID:    textPtr(deref(cat)),
		CompetitionID: comp,
		AuthorID:      author,
		PopularWindow: boolPtr(popular),
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	var items []postDTO
	if popular {
		rows, err := s.q.ListPostsPopular(r.Context(), db.ListPostsPopularParams{
			CategoryID:    textPtr(deref(cat)),
			CompetitionID: comp,
			AuthorID:      author,
			LimitN:        per,
			OffsetN:       offset,
		})
		if err != nil {
			MapDBError(w, err)
			return
		}
		liked := s.likedSet(r.Context(), idsFromPopular(rows))
		items = make([]postDTO, 0, len(rows))
		for _, row := range rows {
			items = append(items, postFromPopular(row, liked[row.ID], true))
		}
	} else {
		rows, err := s.q.ListPostsNew(r.Context(), db.ListPostsNewParams{
			CategoryID:    textPtr(deref(cat)),
			CompetitionID: comp,
			AuthorID:      author,
			LimitN:        per,
			OffsetN:       offset,
		})
		if err != nil {
			MapDBError(w, err)
			return
		}
		liked := s.likedSet(r.Context(), idsFromNew(rows))
		items = make([]postDTO, 0, len(rows))
		for _, row := range rows {
			items = append(items, postFromList(row, liked[row.ID], true))
		}
	}
	JSON(w, http.StatusOK, map[string]any{"posts": items, "meta": meta{Page: int(page), PerPage: int(per), Total: count}})
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func idsFromNew(rows []db.ListPostsNewRow) []uuid.UUID {
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

func idsFromPopular(rows []db.ListPostsPopularRow) []uuid.UUID {
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

func (s *Server) likedSet(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	uid, ok := UserID(ctx)
	if !ok || len(ids) == 0 {
		return out
	}
	rows, err := s.q.ListLikedPostIDs(ctx, db.ListLikedPostIDsParams{UserID: uid, PostIds: ids})
	if err != nil {
		return out
	}
	for _, id := range rows {
		out[id] = true
	}
	return out
}

func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	row, err := s.q.GetPost(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	liked := false
	if uid, ok := UserID(r.Context()); ok {
		ok2, err := s.q.LikedByUser(r.Context(), db.LikedByUserParams{PostID: id, UserID: uid})
		if err == nil {
			liked = ok2
		}
	}
	JSON(w, http.StatusOK, postFromGet(row, liked))
}

type postReq struct {
	Title         string  `json:"title"`
	Body          string  `json:"body"`
	CategoryID    string  `json:"category_id"`
	CompetitionID *string `json:"competition_id"`
}

func (s *Server) parsePostReq(w http.ResponseWriter, r *http.Request) (db.CreatePostParams, bool) {
	var req postReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return db.CreatePostParams{}, false
	}
	fields := map[string]string{}
	if e := Title(req.Title); e != "" {
		fields["title"] = e
	}
	if e := Body(req.Body, 10000); e != "" {
		fields["body"] = e
	}
	if e := Category(req.CategoryID); e != "" {
		fields["category_id"] = e
	}
	var comp *uuid.UUID
	if req.CompetitionID != nil && strings.TrimSpace(*req.CompetitionID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.CompetitionID))
		if err != nil {
			fields["competition_id"] = "大会IDが不正です"
		} else {
			comp = &id
		}
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return db.CreatePostParams{}, false
	}
	return db.CreatePostParams{
		Title:         strings.TrimSpace(req.Title),
		Body:          strings.TrimSpace(req.Body),
		CategoryID:    req.CategoryID,
		CompetitionID: comp,
	}, true
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	params, ok := s.parsePostReq(w, r)
	if !ok {
		return
	}
	params.ID = uuid.New()
	params.AuthorID = p.ID
	row, err := s.q.CreatePost(r.Context(), params)
	if err != nil {
		MapDBError(w, err)
		return
	}
	full, err := s.q.GetPost(r.Context(), row.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, postFromGet(full, false))
}

func (s *Server) updatePost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	params, ok := s.parsePostReq(w, r)
	if !ok {
		return
	}
	_, err := s.q.UpdatePost(r.Context(), db.UpdatePostParams{
		ID:            id,
		AuthorID:      p.ID,
		CategoryID:    params.CategoryID,
		CompetitionID: params.CompetitionID,
		Title:         params.Title,
		Body:          params.Body,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusForbidden, "forbidden", "編集できません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	full, err := s.q.GetPost(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, postFromGet(full, false))
}

func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.SoftDeletePost(r.Context(), db.SoftDeletePostParams{ID: id, AuthorID: p.ID})
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

func (s *Server) likePost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.q.GetPost(r.Context(), id); err != nil {
		MapDBError(w, err)
		return
	}
	n, err := s.q.InsertLike(r.Context(), db.InsertLikeParams{PostID: id, UserID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n > 0 {
		_ = s.q.IncrementLikeCount(r.Context(), id)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unlikePost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.DeleteLike(r.Context(), db.DeleteLikeParams{PostID: id, UserID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n > 0 {
		_ = s.q.DecrementLikeCount(r.Context(), id)
	}
	w.WriteHeader(http.StatusNoContent)
}

func commentJSON(id, postID, authorID uuid.UUID, body, nickname string, created, updated interface{}) map[string]any {
	return map[string]any{
		"id":         id,
		"post_id":    postID,
		"body":       body,
		"author":     authorDTO{ID: authorID, Nickname: nickname},
		"created_at": created,
		"updated_at": updated,
	}
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountComments(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListComments(r.Context(), db.ListCommentsParams{PostID: id, Limit: per, Offset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":         row.ID,
			"post_id":    row.PostID,
			"body":       row.Body,
			"author":     authorDTO{ID: row.AuthorID, Nickname: row.AuthorNickname},
			"created_at": row.CreatedAt,
			"updated_at": row.UpdatedAt,
		})
	}
	JSON(w, http.StatusOK, map[string]any{"comments": out, "meta": meta{Page: int(page), PerPage: int(per), Total: total}})
}

type commentReq struct {
	Body string `json:"body"`
}

func (s *Server) createComment(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	postID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.q.GetPost(r.Context(), postID); err != nil {
		MapDBError(w, err)
		return
	}
	var req commentReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if e := Body(req.Body, 2000); e != "" {
		FieldError(w, map[string]string{"body": e})
		return
	}
	c, err := s.q.CreateComment(r.Context(), db.CreateCommentParams{
		ID:       uuid.New(),
		PostID:   postID,
		AuthorID: p.ID,
		Body:     strings.TrimSpace(req.Body),
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	_ = s.q.IncrementCommentCount(r.Context(), postID)
	JSON(w, http.StatusCreated, commentJSON(c.ID, c.PostID, c.AuthorID, c.Body, p.Nickname, c.CreatedAt, c.UpdatedAt))
}

func (s *Server) updateComment(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req commentReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if e := Body(req.Body, 2000); e != "" {
		FieldError(w, map[string]string{"body": e})
		return
	}
	c, err := s.q.UpdateComment(r.Context(), db.UpdateCommentParams{
		ID:       id,
		AuthorID: p.ID,
		Body:     strings.TrimSpace(req.Body),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusForbidden, "forbidden", "編集できません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, commentJSON(c.ID, c.PostID, c.AuthorID, c.Body, p.Nickname, c.CreatedAt, c.UpdatedAt))
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	c, err := s.q.GetComment(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	n, err := s.q.SoftDeleteComment(r.Context(), db.SoftDeleteCommentParams{ID: id, AuthorID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusForbidden, "forbidden", "削除できません")
		return
	}
	_ = s.q.DecrementCommentCount(r.Context(), c.PostID)
	w.WriteHeader(http.StatusNoContent)
}

type reportReq struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	Detail     string `json:"detail"`
}

func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req reportReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	if e := TargetType(req.TargetType); e != "" {
		fields["target_type"] = e
	}
	tid, err := uuid.Parse(req.TargetID)
	if err != nil {
		fields["target_id"] = "IDが不正です"
	}
	if e := ReportReason(req.Reason); e != "" {
		fields["reason"] = e
	}
	if e := Bio(req.Detail); e != "" {
		fields["detail"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	row, err := s.q.CreateReport(r.Context(), db.CreateReportParams{
		ID:         uuid.New(),
		ReporterID: p.ID,
		TargetType: req.TargetType,
		TargetID:   tid,
		Reason:     req.Reason,
		Detail:     strings.TrimSpace(req.Detail),
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, row)
}

func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountReports(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListReports(r.Context(), db.ListReportsParams{Limit: per, Offset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"reports": rows, "meta": meta{Page: int(page), PerPage: int(per), Total: total}})
}

func (s *Server) hidePost(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.HidePost(r.Context(), id)
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

func (s *Server) hideComment(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.HideComment(r.Context(), id)
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
