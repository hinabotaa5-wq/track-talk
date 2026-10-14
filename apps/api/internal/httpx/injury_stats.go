package httpx

import (
	"math"
	"net/http"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// minInjuryStatSample is the fewest reports a stage needs before its median is shown,
// so a single person's recovery is not read as a typical one.
const minInjuryStatSample = 5

type injuryStatsDTO struct {
	MinSample int                 `json:"min_sample"`
	Parts     []injuryPartStatDTO `json:"parts"`
}

type injuryPartStatDTO struct {
	BodyPart      string               `json:"body_part"`
	BodyPartLabel string               `json:"body_part_label"`
	Posts         int64                `json:"posts"`
	Stages        []injuryStageStatDTO `json:"stages"`
}

type injuryStageStatDTO struct {
	Stage      string `json:"stage"`
	StageLabel string `json:"stage_label"`
	Reached    int64  `json:"reached"`
	// MedianDays is days from the post to the first report of this stage; nil below the sample minimum.
	MedianDays *int `json:"median_days"`
}

// buildInjuryStats lays the rows out in the order of the body-part and stage lists,
// skipping parts with no posts and stages nobody has reached.
func buildInjuryStats(parts []db.CountInjuryPostsByPartRow, stages []db.InjuryStageStatsRow) injuryStatsDTO {
	posts := make(map[string]int64, len(parts))
	for _, row := range parts {
		posts[row.BodyPart] = row.Posts
	}
	byPartStage := make(map[string]map[string]db.InjuryStageStatsRow)
	for _, row := range stages {
		if byPartStage[row.BodyPart] == nil {
			byPartStage[row.BodyPart] = make(map[string]db.InjuryStageStatsRow)
		}
		byPartStage[row.BodyPart][row.Stage] = row
	}

	out := injuryStatsDTO{MinSample: minInjuryStatSample, Parts: []injuryPartStatDTO{}}
	for _, part := range injuryBodyParts {
		if posts[part.Code] == 0 {
			continue
		}
		item := injuryPartStatDTO{
			BodyPart:      part.Code,
			BodyPartLabel: part.Label,
			Posts:         posts[part.Code],
			Stages:        []injuryStageStatDTO{},
		}
		for _, stage := range injuryStages {
			row, ok := byPartStage[part.Code][stage.Code]
			if !ok || row.Reached == 0 {
				continue
			}
			s := injuryStageStatDTO{Stage: stage.Code, StageLabel: stage.Label, Reached: row.Reached}
			if row.Reached >= minInjuryStatSample {
				days := int(math.Round(math.Max(row.MedianDays, 0)))
				s.MedianDays = &days
			}
			item.Stages = append(item.Stages, s)
		}
		out.Parts = append(out.Parts, item)
	}
	return out
}

func (s *Server) injuryStats(w http.ResponseWriter, r *http.Request) {
	parts, err := s.q.CountInjuryPostsByPart(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	stages, err := s.q.InjuryStageStats(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, buildInjuryStats(parts, stages))
}
