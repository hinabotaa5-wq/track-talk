package httpx

import (
	"context"
	"net/http"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 「データ」タブ。ケガ共有のグループで集まった回答(ケガの前に変わったこと・そのときしていたこと・場所・種目)と、
// 復帰までにやったことを、全体とケガごとに集計して見せる。ここでは何も集めず、閲覧だけ。
// 個々の回答や一言は返さない(一言はグループの公開ページにある)。集計は5人そろったものだけ。
type injuryDataDTO struct {
	MinPeople int                    `json:"min_people"`
	Overall   injuryPublicGroupDTO   `json:"overall"`
	Groups    []injuryPublicGroupDTO `json:"groups"`
}

// injuryHindsightCounts は、「どうすればよかったか」に答えた人数と、選択肢ごとの人数。
type injuryHindsightCounts struct {
	people int
	counts map[string]int
}

func (h injuryHindsightCounts) tally() injuryTallyDTO {
	return injuryTally(injuryHindsights, h.counts, h.people)
}

// loadInjuryHindsight は、part のグループの回答を数える。part が空なら全部のグループ。
func (s *Server) loadInjuryHindsight(ctx context.Context, part string) (injuryHindsightCounts, error) {
	out := injuryHindsightCounts{counts: map[string]int{}}
	rows, err := s.pool.Query(ctx, `SELECT codes FROM injury_group_hindsight WHERE $1 = '' OR body_part = $1`, part)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var codes []string
		if err := rows.Scan(&codes); err != nil {
			return out, err
		}
		out.people++
		for _, code := range codes {
			out.counts[code]++
		}
	}
	return out, rows.Err()
}

func (s *Server) injuryData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := injuryDataDTO{MinPeople: injuryPublicMinPeople, Groups: []injuryPublicGroupDTO{}}
	var allCounts []db.CountInjuryGroupMembersByStageRow
	var allAnswers []db.ListInjuryGroupAnswersRow
	var allChanges []db.ListInjuryGroupStageChangesRow
	for _, part := range injuryBodyParts {
		counts, err := s.q.CountInjuryGroupMembersByStage(ctx, part.Code)
		if err != nil {
			MapDBError(w, err)
			return
		}
		answers, err := s.q.ListInjuryGroupAnswers(ctx, part.Code)
		if err != nil {
			MapDBError(w, err)
			return
		}
		changes, err := s.q.ListInjuryGroupStageChanges(ctx, part.Code)
		if err != nil {
			MapDBError(w, err)
			return
		}
		allCounts = append(allCounts, counts...)
		allAnswers = append(allAnswers, answers...)
		allChanges = append(allChanges, changes...)
		group := buildPublicInjuryGroup(part.Code, counts, answers, changes, nil)
		if group.Members == 0 {
			continue
		}
		group.Recovery = []injuryRecoveryDTO{}
		hindsight, err := s.loadInjuryHindsight(ctx, part.Code)
		if err != nil {
			MapDBError(w, err)
			return
		}
		group.Hindsight = hindsight.tally()
		out.Groups = append(out.Groups, group)
	}
	out.Overall = buildPublicInjuryGroup("", allCounts, allAnswers, allChanges, nil)
	out.Overall.BodyPartLabel = "すべてのケガ"
	out.Overall.Recovery = []injuryRecoveryDTO{}
	all, err := s.loadInjuryHindsight(ctx, "")
	if err != nil {
		MapDBError(w, err)
		return
	}
	out.Overall.Hindsight = all.tally()
	JSON(w, http.StatusOK, out)
}
