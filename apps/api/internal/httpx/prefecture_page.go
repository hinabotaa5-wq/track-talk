package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 県ページ: その県のこれからの大会と、クラブチームの募集を1ページに集める。
// 県ごとの掲示板は作らない(人のいない県が空のまま残るため)。
const prefecturePageItems = 10

func prefectureNameOf(code string) string {
	for _, item := range prefectureList {
		if item.code == code {
			return item.name
		}
	}
	return ""
}

func (s *Server) getPrefecturePage(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if !validPrefecture(code) {
		Error(w, http.StatusNotFound, "not_found", "都道府県が見つかりません")
		return
	}
	ctx := r.Context()
	today := tokyoToday()
	competitions, err := s.q.ListUpcomingPrefectureCompetitions(ctx, db.ListUpcomingPrefectureCompetitionsParams{
		Prefecture: code, Today: today, RowLimit: prefecturePageItems,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	competitionTotal, err := s.q.CountUpcomingPrefectureCompetitions(ctx, db.CountUpcomingPrefectureCompetitionsParams{Prefecture: code, Today: today})
	if err != nil {
		MapDBError(w, err)
		return
	}
	recruitments, err := s.q.ListPrefectureRecruitments(ctx, db.ListPrefectureRecruitmentsParams{Prefecture: code, RowLimit: prefecturePageItems})
	if err != nil {
		MapDBError(w, err)
		return
	}
	recruitmentTotal, err := s.q.CountPrefectureRecruitments(ctx, code)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if competitions == nil {
		competitions = []db.ListUpcomingPrefectureCompetitionsRow{}
	}
	if recruitments == nil {
		recruitments = []db.ListPrefectureRecruitmentsRow{}
	}
	JSON(w, http.StatusOK, map[string]any{
		"prefecture":        map[string]string{"code": code, "name": prefectureNameOf(code)},
		"competitions":      competitions,
		"competition_total": competitionTotal,
		"recruitments":      recruitments,
		"recruitment_total": recruitmentTotal,
	})
}
