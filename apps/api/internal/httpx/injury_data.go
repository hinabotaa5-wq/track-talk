package httpx

import (
	"context"
	"net/http"
	"sort"
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
	out.Overall = injuryOverall(out.Groups)
	JSON(w, http.StatusOK, out)
}

// injuryOverall は、全体の集計を、ケガごとに公開している集計(5人以上)だけを足して作る。
// 5人に満たないグループの回答を混ぜると、全体から公開しているグループの分を引くだけで、その人の回答がわかってしまう。
func injuryOverall(groups []injuryPublicGroupDTO) injuryPublicGroupDTO {
	out := buildPublicInjuryGroup("", nil, nil, nil, nil)
	out.BodyPartLabel = "すべてのケガ"
	out.Recovery = []injuryRecoveryDTO{}
	pick := []func(*injuryPublicGroupDTO) *injuryTallyDTO{
		func(g *injuryPublicGroupDTO) *injuryTallyDTO { return &g.Places },
		func(g *injuryPublicGroupDTO) *injuryTallyDTO { return &g.Activities },
		func(g *injuryPublicGroupDTO) *injuryTallyDTO { return &g.PreChanges },
		func(g *injuryPublicGroupDTO) *injuryTallyDTO { return &g.Events },
		func(g *injuryPublicGroupDTO) *injuryTallyDTO { return &g.Actions },
		func(g *injuryPublicGroupDTO) *injuryTallyDTO { return &g.Hindsight },
	}
	for _, field := range pick {
		total := field(&out)
		byCode := map[string]int{}
		labels := []injuryCountDTO{}
		for i := range groups {
			tally := field(&groups[i])
			if tally.People < injuryPublicMinPeople {
				continue
			}
			total.People += tally.People
			for _, item := range tally.Items {
				if _, seen := byCode[item.Code]; !seen {
					labels = append(labels, injuryCountDTO{Code: item.Code, Label: item.Label})
				}
				byCode[item.Code] += item.People
			}
		}
		total.Items = []injuryCountDTO{}
		for _, item := range labels {
			item.People = byCode[item.Code]
			total.Items = append(total.Items, item)
		}
		sort.SliceStable(total.Items, func(i, j int) bool { return total.Items[i].People > total.Items[j].People })
	}
	for _, g := range groups {
		out.Members += g.Members
	}
	return out
}
