package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

type recruitmentInput struct {
	Prefectures []string
	Venue       string
	Schedule    string
	GenderRatio string
	AverageAge  string
	RecruitAge  string
	Links       []string
	PhotoIDs    []string
}

func textLimit(value string, max int, emptyMsg, longMsg string, required bool) (string, string) {
	value = strings.TrimSpace(value)
	n := utf8.RuneCountInString(value)
	if n == 0 && required {
		return "", emptyMsg
	}
	if n > max {
		return "", longMsg
	}
	return value, ""
}

func linkURL(value string) string {
	if utf8.RuneCountInString(value) > 500 {
		return "URLは500文字以内で入力してください"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "httpsのURLを入力してください"
	}
	return ""
}

func normalizeRecruitment(req postReq) (recruitmentInput, map[string]string) {
	fields := map[string]string{}
	var in recruitmentInput
	venue, errMsg := textLimit(req.Venue, 80, "", "活動場所は80文字以内で入力してください", false)
	if errMsg != "" {
		fields["venue"] = errMsg
	}
	schedule, errMsg := textLimit(req.Schedule, 80, "活動時間を入力してください", "活動時間は80文字以内で入力してください", true)
	if errMsg != "" {
		fields["schedule"] = errMsg
	}
	gender, errMsg := textLimit(req.GenderRatio, 80, "", "男女比は80文字以内で入力してください", false)
	if errMsg != "" {
		fields["gender_ratio"] = errMsg
	}
	average, errMsg := textLimit(req.AverageAge, 80, "", "平均年齢は80文字以内で入力してください", false)
	if errMsg != "" {
		fields["average_age"] = errMsg
	}
	recruitAge, errMsg := textLimit(req.RecruitAge, 80, "", "募集年齢は80文字以内で入力してください", false)
	if errMsg != "" {
		fields["recruit_age"] = errMsg
	}
	prefectures, prefErr := normalizePrefectures(req.Prefectures)
	if prefErr != "" {
		fields["prefectures"] = prefErr
	}
	links, linkErr := normalizeLinks(req.Links)
	if linkErr != "" {
		fields["links"] = linkErr
	}
	photos, photoErr := normalizePhotoIDs(req.ImagePublicIDs)
	if photoErr != "" {
		fields["image_public_ids"] = photoErr
	}
	in = recruitmentInput{
		Prefectures: prefectures,
		Venue:       venue,
		Schedule:    schedule,
		GenderRatio: gender,
		AverageAge:  average,
		RecruitAge:  recruitAge,
		Links:       links,
		PhotoIDs:    photos,
	}
	return in, fields
}

func normalizePrefectures(in []string) ([]string, string) {
	if len(in) == 0 {
		return nil, "都道府県を1つ以上選んでください"
	}
	if len(in) > len(prefectureList) {
		return nil, "都道府県が多すぎます"
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		code := strings.TrimSpace(raw)
		if !validPrefecture(code) {
			return nil, "都道府県が不正です"
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	if len(out) == 0 {
		return nil, "都道府県を1つ以上選んでください"
	}
	return out, ""
}

func normalizeLinks(in []string) ([]string, string) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if errMsg := linkURL(value); errMsg != "" {
			return nil, errMsg
		}
		out = append(out, value)
	}
	if len(out) > 4 {
		return nil, "URLは4件までです"
	}
	return out, ""
}

func normalizePhotoIDs(in []string) ([]string, string) {
	if len(in) > 1 {
		return nil, "写真は1枚までです"
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, "写真IDが不正です"
		}
		if errMsg := ImagePublicID(id); errMsg != "" {
			return nil, errMsg
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, ""
}

func (s *Server) photosReady(w http.ResponseWriter, r *http.Request, ids []string) bool {
	for _, id := range ids {
		if err := s.cloud.ImageReady(r.Context(), id); err != nil {
			writeAssetReadyError(w, err, "写真")
			return false
		}
	}
	return true
}

func saveRecruitment(ctx context.Context, q *db.Queries, postID uuid.UUID, in recruitmentInput) error {
	if err := q.UpsertRecruitmentDetail(ctx, db.UpsertRecruitmentDetailParams{
		PostID:      postID,
		Venue:       in.Venue,
		Schedule:    in.Schedule,
		GenderRatio: in.GenderRatio,
		AverageAge:  in.AverageAge,
		RecruitAge:  in.RecruitAge,
	}); err != nil {
		return err
	}
	if err := q.DeleteRecruitmentPrefectures(ctx, postID); err != nil {
		return err
	}
	for i, code := range in.Prefectures {
		if err := q.InsertRecruitmentPrefecture(ctx, db.InsertRecruitmentPrefectureParams{
			PostID:     postID,
			Position:   int32(i),
			Prefecture: code,
		}); err != nil {
			return err
		}
	}
	if err := q.DeleteRecruitmentLinks(ctx, postID); err != nil {
		return err
	}
	for i, link := range in.Links {
		if err := q.InsertRecruitmentLink(ctx, db.InsertRecruitmentLinkParams{
			PostID:   postID,
			Position: int32(i),
			Url:      link,
		}); err != nil {
			return err
		}
	}
	if err := q.DeleteRecruitmentPhotos(ctx, postID); err != nil {
		return err
	}
	for i, id := range in.PhotoIDs {
		if err := q.InsertRecruitmentPhoto(ctx, db.InsertRecruitmentPhotoParams{
			PostID:             postID,
			Position:           int32(i),
			CloudinaryPublicID: id,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) createRecruitment(w http.ResponseWriter, r *http.Request, author db.Profile, params db.CreatePostParams, in recruitmentInput) {
	if !s.photosReady(w, r, in.PhotoIDs) {
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	params.ID = uuid.New()
	params.AuthorID = author.ID
	row, err := q.CreatePost(r.Context(), params)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := saveRecruitment(r.Context(), q, row.ID, in); err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.InsertRecruitmentPostNotifications(r.Context(), db.InsertRecruitmentPostNotificationsParams{
		PostID:   row.ID,
		AuthorID: author.ID,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, row.ID, http.StatusCreated)
}

func (s *Server) updateRecruitment(w http.ResponseWriter, r *http.Request, author db.Profile, id uuid.UUID) {
	params, in, _, ok := s.parsePostReq(w, r, "recruitment")
	if !ok {
		return
	}
	if !s.photosReady(w, r, in.PhotoIDs) {
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	if _, err := q.UpdatePost(r.Context(), db.UpdatePostParams{
		ID:       id,
		AuthorID: author.ID,
		Title:    params.Title,
		Body:     params.Body,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusForbidden, "forbidden", "編集できません")
			return
		}
		MapDBError(w, err)
		return
	}
	if err := saveRecruitment(r.Context(), q, id, in); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, id, http.StatusOK)
}

func (s *Server) writePost(w http.ResponseWriter, r *http.Request, id uuid.UUID, status int) {
	full, err := s.q.GetPost(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	item, err := s.attachRecruitment(r.Context(), withMine(r, s.present(postFromGet(full, false))))
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
	JSON(w, status, item)
}

func (s *Server) attachRecruitment(ctx context.Context, item postDTO) (postDTO, error) {
	if item.Kind != "recruitment" {
		return item, nil
	}
	detail, err := s.q.GetRecruitmentDetail(ctx, item.ID)
	if err != nil {
		return item, err
	}
	prefectures, err := s.q.ListRecruitmentPrefectures(ctx, item.ID)
	if err != nil {
		return item, err
	}
	links, err := s.q.ListRecruitmentLinks(ctx, item.ID)
	if err != nil {
		return item, err
	}
	photos, err := s.q.ListRecruitmentPhotos(ctx, item.ID)
	if err != nil {
		return item, err
	}
	if prefectures == nil {
		prefectures = []string{}
	}
	if links == nil {
		links = []string{}
	}
	photoDTOs := make([]recruitmentPhotoDTO, 0, len(photos))
	for _, photo := range photos {
		photoDTOs = append(photoDTOs, recruitmentPhotoDTO{
			PublicID: photo.CloudinaryPublicID,
			URL:      s.cloud.ImageURL(photo.CloudinaryPublicID),
		})
	}
	if len(photoDTOs) > 0 {
		item.ImagePublicID = photoDTOs[0].PublicID
		item = s.present(item)
	}
	item.Recruitment = &recruitmentDTO{
		Prefectures: prefectures,
		Venue:       detail.Venue,
		Schedule:    detail.Schedule,
		GenderRatio: detail.GenderRatio,
		AverageAge:  detail.AverageAge,
		RecruitAge:  detail.RecruitAge,
		Links:       links,
		Photos:      photoDTOs,
	}
	return item, nil
}

func (s *Server) attachRecruitmentCovers(ctx context.Context, items []postDTO) error {
	ids := make([]uuid.UUID, 0)
	for _, item := range items {
		if item.Kind == "recruitment" {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ListRecruitmentCovers(ctx, ids)
	if err != nil {
		return err
	}
	covers := map[uuid.UUID]string{}
	for _, row := range rows {
		covers[row.PostID] = row.CloudinaryPublicID
	}
	for i := range items {
		id := covers[items[i].ID]
		if id == "" {
			continue
		}
		items[i].ImagePublicID = id
		items[i] = s.present(items[i])
	}
	return nil
}
