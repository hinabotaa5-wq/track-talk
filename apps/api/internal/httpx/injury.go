package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

type injuryInput struct {
	Mood     string
	BodyPart string
}

type injuryDTO struct {
	Mood          string `json:"mood"`
	MoodLabel     string `json:"mood_label"`
	BodyPart      string `json:"body_part"`
	BodyPartLabel string `json:"body_part_label"`
}

var injuryMoods = []struct{ Code, Label string }{
	{"vent", "吐き出し"},
	{"rehab", "代替トレ"},
}

var injuryBodyParts = []struct{ Code, Label string }{
	{"foot", "足裏・足部"},
	{"ankle", "足首"},
	{"achilles", "アキレス腱"},
	{"calf", "ふくらはぎ"},
	{"shin", "すね"},
	{"knee", "膝"},
	{"quad", "太もも前"},
	{"hamstring", "ハムストリング"},
	{"groin", "股関節・鼠径部"},
	{"hip", "お尻・骨盤"},
	{"lower-back", "腰"},
	{"upper-body", "上半身"},
	{"other", "その他"},
}

func injuryMoodLabel(code string) string {
	for _, item := range injuryMoods {
		if item.Code == code {
			return item.Label
		}
	}
	return code
}

func injuryBodyPartLabel(code string) string {
	for _, item := range injuryBodyParts {
		if item.Code == code {
			return item.Label
		}
	}
	return code
}

func validInjuryMood(code string) bool {
	return injuryMoodLabel(code) != code
}

func validInjuryBodyPart(code string) bool {
	return injuryBodyPartLabel(code) != code
}

func normalizeInjury(req postReq) (injuryInput, map[string]string) {
	fields := map[string]string{}
	mood := strings.TrimSpace(req.Mood)
	if !validInjuryMood(mood) {
		fields["mood"] = "投稿の種類を選んでください"
	}
	part := strings.TrimSpace(req.BodyPart)
	if !validInjuryBodyPart(part) {
		fields["body_part"] = "部位を選んでください"
	}
	return injuryInput{Mood: mood, BodyPart: part}, fields
}

// injuryTitle gives a post without a title the start of its body, so feed cards still have a heading.
func injuryTitle(body string) string {
	flat := strings.Join(strings.Fields(body), " ")
	runes := []rune(flat)
	if len(runes) > 40 {
		return string(runes[:40]) + "…"
	}
	return flat
}

func injuryFromDetail(row db.InjuryDetail) *injuryDTO {
	return &injuryDTO{
		Mood:          row.Mood,
		MoodLabel:     injuryMoodLabel(row.Mood),
		BodyPart:      row.BodyPart,
		BodyPartLabel: injuryBodyPartLabel(row.BodyPart),
	}
}

func saveInjury(ctx context.Context, q *db.Queries, postID uuid.UUID, in injuryInput) error {
	return q.UpsertInjuryDetail(ctx, db.UpsertInjuryDetailParams{
		PostID:   postID,
		Mood:     in.Mood,
		BodyPart: in.BodyPart,
	})
}

func (s *Server) createInjury(w http.ResponseWriter, r *http.Request, author db.Profile, params db.CreatePostParams, in injuryInput) {
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
	if err := saveInjury(r.Context(), q, row.ID, in); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, row.ID, http.StatusCreated)
}

func (s *Server) updateInjury(w http.ResponseWriter, r *http.Request, author db.Profile, id uuid.UUID) {
	params, _, _, in, ok := s.parsePostReq(w, r, "injury")
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
	_, err = q.UpdatePost(r.Context(), db.UpdatePostParams{
		ID:       id,
		AuthorID: author.ID,
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
	if err := saveInjury(r.Context(), q, id, in); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, id, http.StatusOK)
}

func (s *Server) attachInjury(ctx context.Context, item postDTO) (postDTO, error) {
	if item.Kind != "injury" {
		return item, nil
	}
	detail, err := s.q.GetInjuryDetail(ctx, item.ID)
	if err != nil {
		return item, err
	}
	item.Injury = injuryFromDetail(detail)
	return item, nil
}

func (s *Server) attachInjurySummaries(ctx context.Context, items []postDTO) error {
	ids := make([]uuid.UUID, 0)
	for _, item := range items {
		if item.Kind == "injury" {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ListInjuryDetails(ctx, ids)
	if err != nil {
		return err
	}
	byID := map[uuid.UUID]db.InjuryDetail{}
	for _, row := range rows {
		byID[row.PostID] = row
	}
	for i := range items {
		if row, ok := byID[items[i].ID]; ok {
			items[i].Injury = injuryFromDetail(row)
		}
	}
	return nil
}
