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

func boolPtr(v bool) *bool {
	return &v
}

func likeContains(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var b strings.Builder
	for _, r := range raw {
		if r == '\\' || r == '%' || r == '_' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	s := b.String()
	return &s
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
	identity, ok := userIdentity(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
		return db.Profile{}, false
	}
	var nickname *string
	if value := strings.TrimSpace(identity.Nickname); value != "" && Nickname(value) == "" {
		nickname = &value
	}
	p, err := s.q.EnsureProfile(r.Context(), db.EnsureProfileParams{
		ID:             identity.UserID,
		Nickname:       nickname,
		NicknamePublic: nickname != nil && identity.NicknamePublic,
	})
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
		"id":              p.ID,
		"nickname":        p.Nickname,
		"nickname_public": p.NicknamePublic,
		"is_admin":        p.IsAdmin,
		"created_at":      p.CreatedAt,
		"updated_at":      p.UpdatedAt,
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
	Nickname       string `json:"nickname"`
	NicknamePublic bool   `json:"nickname_public"`
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req profileReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	nickname := strings.TrimSpace(req.Nickname)
	if message := Nickname(nickname); message != "" {
		FieldError(w, map[string]string{"nickname": message})
		return
	}
	updated, err := s.q.UpdateProfile(r.Context(), db.UpdateProfileParams{
		ID:             p.ID,
		Nickname:       &nickname,
		NicknamePublic: req.NicknamePublic,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, profileJSON(updated))
}

func (s *Server) listMyPosts(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	s.listPostsFiltered(w, r, "new", &p.ID, nil)
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	var kind *string
	if value := strings.TrimSpace(r.URL.Query().Get("kind")); value == "recruitment" {
		kind = &value
	}
	s.listPostsFiltered(w, r, r.URL.Query().Get("sort"), nil, kind)
}

func withMine(r *http.Request, p postDTO) postDTO {
	uid, ok := UserID(r.Context())
	p.Mine = ok && uid == p.authorID
	return p
}

func (s *Server) listPostsFiltered(w http.ResponseWriter, r *http.Request, sort string, author *uuid.UUID, kind *string) {
	page, per, offset := s.queryPage(r)
	popular := sort == "popular"
	keyword, codes := searchArgs(r.URL.Query().Get("q"))
	count, err := s.q.CountPosts(r.Context(), db.CountPostsParams{
		AuthorID:        author,
		Kind:            kind,
		PopularWindow:   boolPtr(popular),
		Keyword:         keyword,
		PrefectureCodes: codes,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	var items []postDTO
	if popular {
		rows, err := s.q.ListPostsPopular(r.Context(), db.ListPostsPopularParams{
			AuthorID:        author,
			Kind:            kind,
			Keyword:         keyword,
			PrefectureCodes: codes,
			LimitN:          per,
			OffsetN:         offset,
		})
		if err != nil {
			MapDBError(w, err)
			return
		}
		ids := idsFromPopular(rows)
		liked := s.likedSet(r.Context(), ids)
		saved := s.savedSet(r.Context(), ids)
		items = make([]postDTO, 0, len(rows))
		for _, row := range rows {
			item := withMine(r, s.present(postFromPopular(row, liked[row.ID], true)))
			item.SavedByMe = saved[row.ID]
			items = append(items, item)
		}
	} else {
		rows, err := s.q.ListPostsNew(r.Context(), db.ListPostsNewParams{
			AuthorID:        author,
			Kind:            kind,
			Keyword:         keyword,
			PrefectureCodes: codes,
			LimitN:          per,
			OffsetN:         offset,
		})
		if err != nil {
			MapDBError(w, err)
			return
		}
		ids := idsFromNew(rows)
		liked := s.likedSet(r.Context(), ids)
		saved := s.savedSet(r.Context(), ids)
		items = make([]postDTO, 0, len(rows))
		for _, row := range rows {
			item := withMine(r, s.present(postFromList(row, liked[row.ID], true)))
			item.SavedByMe = saved[row.ID]
			items = append(items, item)
		}
	}
	if err := s.attachRecruitmentCovers(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	if err := s.attachPracticeSummaries(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"posts": items, "meta": meta{Page: int(page), PerPage: int(per), Total: count}})
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

func (s *Server) savedSet(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	uid, ok := UserID(ctx)
	if !ok || len(ids) == 0 {
		return out
	}
	rows, err := s.q.ListSavedPostIDs(ctx, db.ListSavedPostIDsParams{UserID: uid, PostIds: ids})
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
	saved := false
	if uid, ok := UserID(r.Context()); ok {
		ok2, err := s.q.LikedByUser(r.Context(), db.LikedByUserParams{PostID: id, UserID: uid})
		if err == nil {
			liked = ok2
		}
		ok2, err = s.q.SavedByUser(r.Context(), db.SavedByUserParams{PostID: id, UserID: uid})
		if err == nil {
			saved = ok2
		}
	}
	item := withMine(r, s.present(postFromGet(row, liked)))
	item, err = s.attachRecruitment(r.Context(), item)
	if err != nil {
		MapDBError(w, err)
		return
	}
	item, err = s.attachPractice(r.Context(), item)
	if err != nil {
		MapDBError(w, err)
		return
	}
	item, err = s.attachPracticeViewer(r, item)
	if err != nil {
		MapDBError(w, err)
		return
	}
	item.SavedByMe = saved
	JSON(w, http.StatusOK, item)
}

type postReq struct {
	Title               string   `json:"title"`
	Body                string   `json:"body"`
	Kind                string   `json:"kind"`
	YouTubeVideoID      string   `json:"youtube_video_id"`
	YouTubeThumbnailURL string   `json:"youtube_thumbnail_url"`
	VideoPublicID       string   `json:"video_public_id"`
	ImagePublicID       string   `json:"image_public_id"`
	Prefectures         []string `json:"prefectures"`
	Venue               string   `json:"venue"`
	Schedule            string   `json:"schedule"`
	GenderRatio         string   `json:"gender_ratio"`
	AverageAge          string   `json:"average_age"`
	RecruitAge          string   `json:"recruit_age"`
	Links               []string `json:"links"`
	ImagePublicIDs      []string `json:"image_public_ids"`
	StartsAt            string   `json:"starts_at"`
	Event               string   `json:"event"`
	EventOther          string   `json:"event_other"`
	Capacity            int      `json:"capacity"`
}

func (s *Server) parsePostReq(w http.ResponseWriter, r *http.Request, forceKind string) (db.CreatePostParams, recruitmentInput, practiceInput, bool) {
	var req postReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return db.CreatePostParams{}, recruitmentInput{}, practiceInput{}, false
	}
	if forceKind != "" {
		req.Kind = forceKind
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "text"
	}
	fields := map[string]string{}
	if kind != "practice" {
		if e := Title(req.Title); e != "" {
			fields["title"] = e
		}
	}
	if e := PostBody(req.Body); e != "" {
		fields["body"] = e
	}
	if e := Kind(kind); e != "" {
		fields["kind"] = e
	}
	var youtubeID, youtubeThumb, publicID *string
	var recruitment recruitmentInput
	var practice practiceInput
	switch kind {
	case "text":
		if strings.TrimSpace(req.YouTubeVideoID) != "" || strings.TrimSpace(req.YouTubeThumbnailURL) != "" || strings.TrimSpace(req.VideoPublicID) != "" {
			fields["kind"] = "文字の投稿に動画情報は付けられません"
		}
		imageID := strings.TrimSpace(req.ImagePublicID)
		if imageID != "" {
			if e := ImagePublicID(imageID); e != "" {
				fields["image_public_id"] = e
			}
			publicID = &imageID
		}
	case "youtube":
		id := strings.TrimSpace(req.YouTubeVideoID)
		thumb := strings.TrimSpace(req.YouTubeThumbnailURL)
		if e := YouTubeVideoID(id); e != "" {
			fields["youtube_video_id"] = e
		}
		if e := YouTubeThumbnailURL(thumb); e != "" {
			fields["youtube_thumbnail_url"] = e
		}
		if strings.TrimSpace(req.VideoPublicID) != "" {
			fields["video_public_id"] = "自分の動画とは同時に指定できません"
		}
		if strings.TrimSpace(req.ImagePublicID) != "" {
			fields["image_public_id"] = "写真は文章の投稿に付けてください"
		}
		youtubeID = &id
		youtubeThumb = &thumb
	case "video":
		id := strings.TrimSpace(req.VideoPublicID)
		if e := VideoPublicID(id); e != "" {
			fields["video_public_id"] = e
		}
		if strings.TrimSpace(req.YouTubeVideoID) != "" || strings.TrimSpace(req.YouTubeThumbnailURL) != "" {
			fields["youtube_video_id"] = "YouTubeとは同時に指定できません"
		}
		if strings.TrimSpace(req.ImagePublicID) != "" {
			fields["image_public_id"] = "写真は文章の投稿に付けてください"
		}
		publicID = &id
	case "recruitment":
		if strings.TrimSpace(req.YouTubeVideoID) != "" || strings.TrimSpace(req.YouTubeThumbnailURL) != "" || strings.TrimSpace(req.VideoPublicID) != "" {
			fields["kind"] = "募集の投稿に動画情報は付けられません"
		}
		if strings.TrimSpace(req.ImagePublicID) != "" {
			fields["image_public_id"] = "写真はまとめて指定してください"
		}
		if strings.TrimSpace(req.Title) == "" {
			fields["title"] = "チーム名を入力してください"
		}
		if strings.TrimSpace(req.Body) == "" {
			fields["body"] = "詳細を入力してください"
		}
		parsed, fieldErrs := normalizeRecruitment(req)
		for key, value := range fieldErrs {
			fields[key] = value
		}
		recruitment = parsed
	case "practice":
		if strings.TrimSpace(req.YouTubeVideoID) != "" || strings.TrimSpace(req.YouTubeThumbnailURL) != "" || strings.TrimSpace(req.VideoPublicID) != "" {
			fields["kind"] = "練習相手の募集に動画情報は付けられません"
		}
		if strings.TrimSpace(req.ImagePublicID) != "" || len(req.ImagePublicIDs) > 0 {
			fields["image_public_id"] = "この募集に写真は付けられません"
		}
		body, bodyErr := textLimit(req.Body, 2000, "詳細を入力してください", "詳細は2000文字以内で入力してください", true)
		if bodyErr != "" {
			fields["body"] = bodyErr
		} else {
			req.Body = body
		}
		parsed, fieldErrs := normalizePractice(req)
		for key, value := range fieldErrs {
			fields[key] = value
		}
		practice = parsed
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return db.CreatePostParams{}, recruitmentInput{}, practiceInput{}, false
	}
	title := strings.TrimSpace(req.Title)
	if kind == "practice" {
		title = practiceTitle(practice.Event, practice.EventOther, practice.StartsAt)
	}
	return db.CreatePostParams{
		Title:               title,
		Body:                strings.TrimSpace(req.Body),
		Kind:                kind,
		YoutubeVideoID:      youtubeID,
		YoutubeThumbnailUrl: youtubeThumb,
		CloudinaryPublicID:  publicID,
	}, recruitment, practice, true
}

func textPostHasContent(title, body string, imagePublicID *string) bool {
	if strings.TrimSpace(title) != "" || strings.TrimSpace(body) != "" {
		return true
	}
	return imagePublicID != nil && strings.TrimSpace(*imagePublicID) != ""
}

func writeEmptyTextPostError(w http.ResponseWriter) {
	FieldError(w, map[string]string{
		"body": "タイトル、本文、写真のいずれかを入力してください",
	})
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	params, recruitment, practice, ok := s.parsePostReq(w, r, "")
	if !ok {
		return
	}
	if params.Kind == "recruitment" {
		s.createRecruitment(w, r, p, params, recruitment)
		return
	}
	if params.Kind == "practice" {
		s.createPractice(w, r, p, params, practice)
		return
	}
	if params.Kind == "text" && !textPostHasContent(params.Title, params.Body, params.CloudinaryPublicID) {
		writeEmptyTextPostError(w)
		return
	}
	params.ID = uuid.New()
	params.AuthorID = p.ID
	if params.Kind == "video" {
		publicID := ""
		if params.CloudinaryPublicID != nil {
			publicID = *params.CloudinaryPublicID
		}
		if err := s.cloud.Ready(r.Context(), publicID); err != nil {
			writeVideoReadyError(w, err)
			return
		}
	}
	if params.Kind == "text" && params.CloudinaryPublicID != nil {
		if err := s.cloud.ImageReady(r.Context(), *params.CloudinaryPublicID); err != nil {
			writeAssetReadyError(w, err, "写真")
			return
		}
	}
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
	JSON(w, http.StatusCreated, withMine(r, s.present(postFromGet(full, false))))
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
	existing, err := s.q.GetPost(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if existing.AuthorID != p.ID {
		Error(w, http.StatusForbidden, "forbidden", "編集できません")
		return
	}
	if existing.Kind == "recruitment" {
		s.updateRecruitment(w, r, p, id)
		return
	}
	if existing.Kind == "practice" {
		s.updatePractice(w, r, p, id)
		return
	}
	params, _, _, ok := s.parsePostReq(w, r, "")
	if !ok {
		return
	}
	if existing.Kind == "text" && !textPostHasContent(params.Title, params.Body, existing.CloudinaryPublicID) {
		writeEmptyTextPostError(w)
		return
	}
	_, err = s.q.UpdatePost(r.Context(), db.UpdatePostParams{
		ID:       id,
		AuthorID: p.ID,
		Title:    params.Title,
		Body:     params.Body,
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
	JSON(w, http.StatusOK, withMine(r, s.present(postFromGet(full, false))))
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

func (s *Server) listSaves(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	keyword, codes := searchArgs(r.URL.Query().Get("q"))
	count, err := s.q.CountSavedPosts(r.Context(), db.CountSavedPostsParams{UserID: p.ID, Keyword: keyword, PrefectureCodes: codes})
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListSavedPosts(r.Context(), db.ListSavedPostsParams{
		UserID:          p.ID,
		Keyword:         keyword,
		PrefectureCodes: codes,
		LimitN:          per,
		OffsetN:         offset,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	liked := s.likedSet(r.Context(), ids)
	items := make([]postDTO, 0, len(rows))
	for _, row := range rows {
		item := withMine(r, s.present(postFromList(db.ListPostsNewRow{
			ID:                  row.ID,
			AuthorID:            row.AuthorID,
			Title:               row.Title,
			Body:                row.Body,
			Kind:                row.Kind,
			YoutubeVideoID:      row.YoutubeVideoID,
			YoutubeThumbnailUrl: row.YoutubeThumbnailUrl,
			CloudinaryPublicID:  row.CloudinaryPublicID,
			LikeCount:           row.LikeCount,
			CommentCount:        row.CommentCount,
			CreatedAt:           row.CreatedAt,
			UpdatedAt:           row.UpdatedAt,
		}, liked[row.ID], true)))
		item.SavedByMe = true
		items = append(items, item)
	}
	if err := s.attachRecruitmentCovers(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	if err := s.attachPracticeSummaries(r.Context(), items); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"posts": items, "meta": meta{Page: int(page), PerPage: int(per), Total: count}})
}

func (s *Server) savePost(w http.ResponseWriter, r *http.Request) {
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
	if _, err := s.q.InsertSave(r.Context(), db.InsertSaveParams{PostID: id, UserID: p.ID}); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unsavePost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.q.DeleteSave(r.Context(), db.DeleteSaveParams{PostID: id, UserID: p.ID}); err != nil {
		MapDBError(w, err)
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
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	n, err := q.InsertLike(r.Context(), db.InsertLikeParams{PostID: id, UserID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n > 0 {
		if err := q.IncrementLikeCount(r.Context(), id); err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
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
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	n, err := q.DeleteLike(r.Context(), db.DeleteLikeParams{PostID: id, UserID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n > 0 {
		if err := q.DecrementLikeCount(r.Context(), id); err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func commentJSON(r *http.Request, id, postID, authorID uuid.UUID, body string, created, updated interface{}) map[string]any {
	uid, ok := UserID(r.Context())
	return map[string]any{
		"id":         id,
		"post_id":    postID,
		"body":       body,
		"mine":       ok && uid == authorID,
		"created_at": created,
		"updated_at": updated,
	}
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.q.GetPost(r.Context(), id); err != nil {
		MapDBError(w, err)
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
		out = append(out, commentJSON(r, row.ID, row.PostID, row.AuthorID, row.Body, row.CreatedAt, row.UpdatedAt))
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
	post, err := s.q.GetPost(r.Context(), postID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	allowed, err := s.practiceCommentAllowed(r.Context(), post, p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if !allowed {
		Error(w, http.StatusForbidden, "forbidden", "募集が終了したためコメントできません")
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
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	c, err := q.CreateComment(r.Context(), db.CreateCommentParams{
		ID:       uuid.New(),
		PostID:   postID,
		AuthorID: p.ID,
		Body:     strings.TrimSpace(req.Body),
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.IncrementCommentCount(r.Context(), postID); err != nil {
		MapDBError(w, err)
		return
	}
	if post.AuthorID != p.ID {
		if err := insertNotification(r.Context(), q, post.AuthorID, "comment", postID, &c.ID, nil); err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, commentJSON(r, c.ID, c.PostID, c.AuthorID, c.Body, c.CreatedAt, c.UpdatedAt))
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
	JSON(w, http.StatusOK, commentJSON(r, c.ID, c.PostID, c.AuthorID, c.Body, c.CreatedAt, c.UpdatedAt))
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
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	c, err := q.GetComment(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	n, err := q.SoftDeleteComment(r.Context(), db.SoftDeleteCommentParams{ID: id, AuthorID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusForbidden, "forbidden", "削除できません")
		return
	}
	if err := q.DecrementCommentCount(r.Context(), c.PostID); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
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
	if e := ReportDetail(req.Detail); e != "" {
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

type inquiryReq struct {
	Category string `json:"category"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
}

func (s *Server) createInquiry(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req inquiryReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	fields := map[string]string{}
	if e := InquiryCategory(req.Category); e != "" {
		fields["category"] = e
	}
	if e := InquirySubject(req.Subject); e != "" {
		fields["subject"] = e
	}
	if e := Body(req.Body, 2000); e != "" {
		fields["body"] = e
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	row, err := s.q.CreateInquiry(r.Context(), db.CreateInquiryParams{
		ID:       uuid.New(),
		UserID:   p.ID,
		Category: req.Category,
		Subject:  strings.TrimSpace(req.Subject),
		Body:     strings.TrimSpace(req.Body),
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, row)
}

func (s *Server) listMyInquiries(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountMyInquiries(r.Context(), p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListMyInquiries(r.Context(), db.ListMyInquiriesParams{
		UserID: p.ID,
		Limit:  per,
		Offset: offset,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.Inquiry{}
	}
	JSON(w, http.StatusOK, map[string]any{
		"inquiries": rows,
		"meta":      meta{Page: int(page), PerPage: int(per), Total: total},
	})
}

func (s *Server) listInquiries(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountInquiries(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListInquiries(r.Context(), db.ListInquiriesParams{Limit: per, Offset: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if rows == nil {
		rows = []db.Inquiry{}
	}
	JSON(w, http.StatusOK, map[string]any{
		"inquiries": rows,
		"meta":      meta{Page: int(page), PerPage: int(per), Total: total},
	})
}

func (s *Server) hideComment(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	c, err := q.GetComment(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	n, err := q.HideComment(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	if err := q.DecrementCommentCount(r.Context(), c.PostID); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
