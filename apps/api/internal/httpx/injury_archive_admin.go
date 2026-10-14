package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

const (
	maxArchiveMaskWords = 20
	maxArchiveMaskRunes = 50
	archiveMaskText     = "[伏せ]"
)

var archiveReviewStatuses = map[string]bool{"pending": true, "approved": true, "rejected": true}

type injuryArchiveDTO struct {
	ID            uuid.UUID     `json:"id"`
	BodyPart      string        `json:"body_part"`
	BodyPartLabel string        `json:"body_part_label"`
	InjuredMonth  string        `json:"injured_month"`
	ArchivedMonth string        `json:"archived_month"`
	Timeline      []archiveStep `json:"stage_timeline"`
	Place         string        `json:"place"`
	Situation     string        `json:"situation"`
	Posts         []archivePost `json:"posts"`
	ReviewStatus  string        `json:"review_status"`
}

func injuryArchiveFromRow(row db.InjuryArchive) (injuryArchiveDTO, error) {
	out := injuryArchiveDTO{
		ID:            row.ID,
		BodyPart:      row.BodyPart,
		BodyPartLabel: injuryBodyPartLabel(row.BodyPart),
		InjuredMonth:  row.InjuredMonth.Format("2006-01"),
		ArchivedMonth: row.ArchivedMonth.Format("2006-01"),
		Timeline:      []archiveStep{},
		Place:         row.Place,
		Situation:     row.Situation,
		Posts:         []archivePost{},
		ReviewStatus:  row.ReviewStatus,
	}
	if err := json.Unmarshal(row.StageTimeline, &out.Timeline); err != nil {
		return out, err
	}
	if err := json.Unmarshal(row.Posts, &out.Posts); err != nil {
		return out, err
	}
	return out, nil
}

// maskArchiveText hides each word everywhere in the archived text, longest first so a short word cannot split a long one.
func maskArchiveText(a *injuryArchiveDTO, words []string) {
	for i := 0; i < len(words); i++ {
		for j := i + 1; j < len(words); j++ {
			if utf8.RuneCountInString(words[j]) > utf8.RuneCountInString(words[i]) {
				words[i], words[j] = words[j], words[i]
			}
		}
	}
	mask := func(text string) string {
		for _, word := range words {
			text = strings.ReplaceAll(text, word, archiveMaskText)
		}
		return text
	}
	a.Place = mask(a.Place)
	a.Situation = mask(a.Situation)
	for i := range a.Posts {
		a.Posts[i].Title = mask(a.Posts[i].Title)
		a.Posts[i].Body = mask(a.Posts[i].Body)
		for j := range a.Posts[i].Updates {
			a.Posts[i].Updates[j].Body = mask(a.Posts[i].Updates[j].Body)
		}
	}
}

func (s *Server) listInjuryArchives(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !archiveReviewStatuses[status] {
		FieldError(w, map[string]string{"status": "status は pending / approved / rejected のどれかです"})
		return
	}
	page, per, offset := s.queryPage(r)
	total, err := s.q.CountInjuryArchives(r.Context(), status)
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListInjuryArchives(r.Context(), db.ListInjuryArchivesParams{Status: status, LimitN: per, OffsetN: offset})
	if err != nil {
		MapDBError(w, err)
		return
	}
	items := make([]injuryArchiveDTO, 0, len(rows))
	for _, row := range rows {
		item, err := injuryArchiveFromRow(db.InjuryArchive(row))
		if err != nil {
			MapDBError(w, err)
			return
		}
		items = append(items, item)
	}
	JSON(w, http.StatusOK, map[string]any{
		"archives": items,
		"meta":     meta{Page: int(page), PerPage: int(per), Total: total},
	})
}

func (s *Server) reviewInjuryArchive(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		ReviewStatus string `json:"review_status"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	if !archiveReviewStatuses[req.ReviewStatus] {
		FieldError(w, map[string]string{"review_status": "pending / approved / rejected のどれかを選んでください"})
		return
	}
	n, err := s.q.SetInjuryArchiveReview(r.Context(), db.SetInjuryArchiveReviewParams{ID: id, ReviewStatus: req.ReviewStatus})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "記録が見つかりません")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// maskInjuryArchive hides the given words in an archived record. The record goes back to pending,
// because the reviewer has to read the result again before approving it.
func (s *Server) maskInjuryArchive(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Words []string `json:"words"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	words := make([]string, 0, len(req.Words))
	for _, word := range req.Words {
		if word = strings.TrimSpace(word); word != "" {
			words = append(words, word)
		}
	}
	if len(words) == 0 || len(words) > maxArchiveMaskWords {
		FieldError(w, map[string]string{"words": "伏せる言葉を1〜20個、入力してください"})
		return
	}
	for _, word := range words {
		if utf8.RuneCountInString(word) > maxArchiveMaskRunes {
			FieldError(w, map[string]string{"words": "伏せる言葉は50文字以内にしてください"})
			return
		}
	}
	row, err := s.q.GetInjuryArchive(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	item, err := injuryArchiveFromRow(row)
	if err != nil {
		MapDBError(w, err)
		return
	}
	maskArchiveText(&item, words)
	posts, err := json.Marshal(item.Posts)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if _, err := s.q.UpdateInjuryArchiveText(r.Context(), db.UpdateInjuryArchiveTextParams{
		ID: id, Place: item.Place, Situation: item.Situation, Posts: posts,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	item.ReviewStatus = "pending"
	JSON(w, http.StatusOK, item)
}

// deleteInjuryArchive removes a rejected record, one that could not be made anonymous.
func (s *Server) deleteInjuryArchive(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.DeleteRejectedInjuryArchive(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "削除できるのは、却下した記録だけです")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
