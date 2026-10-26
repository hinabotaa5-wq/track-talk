package httpx

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
)

type injuryHistoryDTO struct {
	Items []injuryHistoryItemDTO `json:"items"`
}

type injuryHistoryItemDTO struct {
	ID            uuid.UUID               `json:"id"`
	Title         string                  `json:"title"`
	BodyPart      string                  `json:"body_part"`
	BodyPartLabel string                  `json:"body_part_label"`
	CreatedAt     time.Time               `json:"created_at"`
	Stages        []injuryHistoryStageDTO `json:"stages"`
}

type injuryHistoryStageDTO struct {
	Stage      string `json:"stage"`
	StageLabel string `json:"stage_label"`
	// Days は、投稿からこの段階の最初の報告までの日数（日単位で切り捨て）。
	Days int `json:"days"`
}

// buildInjuryHistory は、各投稿に段階ごとの最初の報告を、段階の一覧の順に付ける。
func buildInjuryHistory(posts []db.ListMyInjuryPostsRow, reports []db.ListMyInjuryFirstReportsRow) injuryHistoryDTO {
	byPost := make(map[uuid.UUID]map[string]time.Time)
	for _, row := range reports {
		if byPost[row.PostID] == nil {
			byPost[row.PostID] = make(map[string]time.Time)
		}
		byPost[row.PostID][row.Stage] = row.FirstAt
	}
	out := injuryHistoryDTO{Items: make([]injuryHistoryItemDTO, 0, len(posts))}
	for _, post := range posts {
		item := injuryHistoryItemDTO{
			ID:            post.ID,
			Title:         post.Title,
			BodyPart:      post.BodyPart,
			BodyPartLabel: injuryBodyPartLabel(post.BodyPart),
			CreatedAt:     post.CreatedAt,
			Stages:        []injuryHistoryStageDTO{},
		}
		for _, stage := range injuryStages {
			at, ok := byPost[post.ID][stage.Code]
			if !ok {
				continue
			}
			days := int(at.Sub(post.CreatedAt).Hours() / 24)
			if days < 0 {
				days = 0
			}
			item.Stages = append(item.Stages, injuryHistoryStageDTO{Stage: stage.Code, StageLabel: stage.Label, Days: days})
		}
		out.Items = append(out.Items, item)
	}
	return out
}

func (s *Server) myInjuryHistory(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Get("pagination") == "cursor" {
		s.injuryHistoryPage(w, r, me.ID)
		return
	}
	posts, err := s.q.ListMyInjuryPosts(r.Context(), me.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	reports, err := s.q.ListMyInjuryFirstReports(r.Context(), me.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, buildInjuryHistory(posts, reports))
}
