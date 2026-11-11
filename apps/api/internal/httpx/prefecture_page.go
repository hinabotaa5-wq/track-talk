package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 県ページ: その県のクラブチームの募集を1ページに集める。
// 県ごとの掲示板は作らない(人のいない県が空のまま残るため)。
// 県別の大会の一覧は #549 で廃止したため、ここには出さない。
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
	if recruitments == nil {
		recruitments = []db.ListPrefectureRecruitmentsRow{}
	}
	JSON(w, http.StatusOK, map[string]any{
		"prefecture":        map[string]string{"code": code, "name": prefectureNameOf(code)},
		"recruitments":      recruitments,
		"recruitment_total": recruitmentTotal,
	})
}
