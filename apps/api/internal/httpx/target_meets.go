package httpx

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// メモタブの「〇〇大会まであと何日」に使う、本人が目標にセットした大会。1人1大会。
type targetMeetDTO struct {
	Source   string  `json:"source"`
	Ref      string  `json:"ref"`
	Name     string  `json:"name"`
	StartsOn string  `json:"starts_on"`
	EndsOn   *string `json:"ends_on"`
}

func targetMeetJSON(row db.GetTargetMeetRow) targetMeetDTO {
	dto := targetMeetDTO{Source: row.Source, Ref: row.Ref, Name: row.Name, StartsOn: row.StartsOn.Format("2006-01-02")}
	if row.EndsOn != nil {
		end := row.EndsOn.Format("2006-01-02")
		dto.EndsOn = &end
	}
	return dto
}

func (s *Server) getTargetMeet(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	row, err := s.q.GetTargetMeet(r.Context(), me.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		JSON(w, http.StatusOK, map[string]any{"target_meet": nil})
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"target_meet": targetMeetJSON(row)})
}

type targetMeetReq struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
}

// 名前と日付は送られた値を使わず、大会一覧から読み直して写す。
func (s *Server) setTargetMeet(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req targetMeetReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	params := db.SetTargetMeetParams{ProfileID: me.ID, Source: req.Source}
	var err error
	switch req.Source {
	case "major":
		slug, yearText, found := strings.Cut(req.Ref, ":")
		year, convErr := strconv.Atoi(yearText)
		if !found || slug == "" || convErr != nil {
			Error(w, http.StatusNotFound, "not_found", "大会が見つかりません")
			return
		}
		var major db.GetDomesticMajorForTargetRow
		major, err = s.q.GetDomesticMajorForTarget(r.Context(), db.GetDomesticMajorForTargetParams{Slug: slug, Year: int32(year)})
		ends := major.EndsOn
		params.Ref, params.Name, params.StartsOn, params.EndsOn = slug+":"+strconv.Itoa(year), major.Name, major.StartsOn, &ends
	case "competition":
		id, parseErr := uuid.Parse(req.Ref)
		if parseErr != nil {
			Error(w, http.StatusNotFound, "not_found", "大会が見つかりません")
			return
		}
		var competition db.GetCompetitionForTargetRow
		competition, err = s.q.GetCompetitionForTarget(r.Context(), id)
		params.Ref, params.Name, params.StartsOn, params.EndsOn = id.String(), competition.Name, competition.StartsOn, competition.EndsOn
	default:
		Error(w, http.StatusBadRequest, "validation_error", "大会の種類が不正です")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusNotFound, "not_found", "大会が見つかりません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	row, err := s.q.SetTargetMeet(r.Context(), params)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"target_meet": targetMeetJSON(db.GetTargetMeetRow(row))})
}

func (s *Server) deleteTargetMeet(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if err := s.q.DeleteTargetMeet(r.Context(), me.ID); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"target_meet": nil})
}
