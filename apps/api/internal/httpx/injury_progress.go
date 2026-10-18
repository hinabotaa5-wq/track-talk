package httpx

import (
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

type injuryProgressDTO struct {
	Posts     []injuryProgressPostDTO `json:"posts"`
	MinSample int                     `json:"min_sample"`
}

type injuryProgressPostDTO struct {
	PostID        uuid.UUID                `json:"post_id"`
	BodyPart      string                   `json:"body_part"`
	BodyPartLabel string                   `json:"body_part_label"`
	PostedOn      string                   `json:"posted_on"`
	Stages        []injuryProgressStageDTO `json:"stages"`
}

type injuryProgressStageDTO struct {
	Stage      string `json:"stage"`
	StageLabel string `json:"stage_label"`
	// MyDays is whole days from the post to the author's first report of this stage; nil until reached.
	MyDays *int `json:"my_days"`
	// MedianDays is everyone's median for this injury and stage; nil below the sample minimum.
	MedianDays *int `json:"median_days"`
}

// buildInjuryProgress puts each of the author's injury posts next to everyone's medians, stage by stage.
// The first stage (resting) is the post itself: day 0, with no median to compare against.
func buildInjuryProgress(posts []db.ListMyInjuryPostsRow, reports []db.ListMyInjuryFirstReportsRow, stats []db.InjuryStageStatsRow) injuryProgressDTO {
	byPost := make(map[uuid.UUID]map[string]time.Time)
	for _, row := range reports {
		if byPost[row.PostID] == nil {
			byPost[row.PostID] = make(map[string]time.Time)
		}
		byPost[row.PostID][row.Stage] = row.FirstAt
	}
	medians := make(map[string]map[string]*int)
	for _, row := range stats {
		if row.Reached < minInjuryStatSample {
			continue
		}
		if medians[row.BodyPart] == nil {
			medians[row.BodyPart] = make(map[string]*int)
		}
		days := int(math.Round(math.Max(row.MedianDays, 0)))
		medians[row.BodyPart][row.Stage] = &days
	}

	out := injuryProgressDTO{Posts: make([]injuryProgressPostDTO, 0, len(posts)), MinSample: minInjuryStatSample}
	for _, post := range posts {
		item := injuryProgressPostDTO{
			PostID:        post.ID,
			BodyPart:      post.BodyPart,
			BodyPartLabel: injuryBodyPartLabel(post.BodyPart),
			PostedOn:      post.CreatedAt.In(jst.Zone).Format("2006-01-02"),
			Stages:        make([]injuryProgressStageDTO, 0, len(injuryStages)),
		}
		for i, stage := range injuryStages {
			s := injuryProgressStageDTO{Stage: stage.Code, StageLabel: stage.Label}
			if i == 0 {
				zero := 0
				s.MyDays = &zero
			} else {
				if at, ok := byPost[post.ID][stage.Code]; ok {
					days := max(int(at.Sub(post.CreatedAt).Hours()/24), 0)
					s.MyDays = &days
				}
				s.MedianDays = medians[post.BodyPart][stage.Code]
			}
			item.Stages = append(item.Stages, s)
		}
		out.Posts = append(out.Posts, item)
	}
	return out
}

func (s *Server) myInjuryProgress(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
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
	stats, err := s.q.InjuryStageStats(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, buildInjuryProgress(posts, reports, stats))
}
