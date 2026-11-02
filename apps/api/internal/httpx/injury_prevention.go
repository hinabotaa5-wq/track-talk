package httpx

import (
	"context"
	"net/http"
)

// 感覚メモに出す「ケガの予防カード」。見ている人が登録した種目で、ケガ共有のグループの回答を絞って集計する。
// 掛け合わせるのは「種目×1項目」まで。個々の回答は返さず、答えた人が5人に満たない項目は出さない。
//
// 種目で絞る前に、5人に満たないケガのグループは丸ごと外す。「データ」タブの全体の集計と同じ理由で、
// 小さいグループの回答を混ぜると、公開されているケガごとの集計と引き比べて1人の回答がわかってしまうため。
type injuryPreventionDTO struct {
	Event      string         `json:"event"`
	EventLabel string         `json:"event_label"`
	MinPeople  int            `json:"min_people"`
	BodyParts  injuryTallyDTO `json:"body_parts"`
	PreChanges injuryTallyDTO `json:"pre_changes"`
	Hindsight  injuryTallyDTO `json:"hindsight"`
}

func injuryBodyPartOptions() []injuryOption {
	out := make([]injuryOption, 0, len(injuryBodyParts))
	for _, part := range injuryBodyParts {
		out = append(out, injuryOption{part.Code, part.Label})
	}
	return out
}

// injuryPreventionAnswer は、1人分の入るときの答えのうち、予防カードに使うもの。
type injuryPreventionAnswer struct {
	bodyPart   string
	preChanges []string
}

func (s *Server) loadInjuryPreventionAnswers(ctx context.Context, event string) ([]injuryPreventionAnswer, error) {
	// グループの人数は、公開ページの集計と同じく、選択式の答えがある人(severity が入っている人)で数える。
	rows, err := s.pool.Query(ctx, `
WITH answered AS (
    SELECT body_part, pre_changes, event
    FROM injury_group_members
    WHERE severity IS NOT NULL
),
public_parts AS (
    SELECT body_part FROM answered GROUP BY body_part HAVING count(*) >= $2
)
SELECT a.body_part, a.pre_changes
FROM answered a
JOIN public_parts p USING (body_part)
WHERE a.event = $1`, event, injuryPublicMinPeople)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []injuryPreventionAnswer{}
	for rows.Next() {
		var a injuryPreventionAnswer
		if err := rows.Scan(&a.bodyPart, &a.preChanges); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Server) loadInjuryPreventionHindsight(ctx context.Context, event string) (injuryHindsightCounts, error) {
	out := injuryHindsightCounts{counts: map[string]int{}}
	rows, err := s.pool.Query(ctx, `
WITH public_parts AS (
    SELECT body_part FROM injury_group_hindsight GROUP BY body_part HAVING count(*) >= $2
)
SELECT h.codes
FROM injury_group_hindsight h
JOIN public_parts p USING (body_part)
JOIN injury_group_members m ON m.user_id = h.user_id AND m.body_part = h.body_part
WHERE m.event = $1`, event, injuryPublicMinPeople)
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
			// 「とくにない」は予防のヒントにならないので数えない(答えた人数には入れる)。
			if code != "none" {
				out.counts[code]++
			}
		}
	}
	return out, rows.Err()
}

func buildInjuryPrevention(event string, answers []injuryPreventionAnswer, hindsight injuryHindsightCounts) injuryPreventionDTO {
	parts, changes := map[string]int{}, map[string]int{}
	for _, a := range answers {
		parts[a.bodyPart]++
		for _, code := range a.preChanges {
			if code != "none" {
				changes[code]++
			}
		}
	}
	return injuryPreventionDTO{
		Event:      event,
		EventLabel: eventLabel(event),
		MinPeople:  injuryPublicMinPeople,
		BodyParts:  injuryTally(injuryBodyPartOptions(), parts, len(answers)),
		PreChanges: injuryTally(injuryPreChanges, changes, len(answers)),
		Hindsight:  hindsight.tally(),
	}
}

// injuryPrevention は、ログインしていなくても見られる。返すのは公開している集計と同じ粒度の人数だけ。
func (s *Server) injuryPrevention(w http.ResponseWriter, r *http.Request) {
	event := r.URL.Query().Get("event")
	if !validEvent(event) {
		Error(w, http.StatusBadRequest, "invalid_event", "種目が正しくありません")
		return
	}
	answers, err := s.loadInjuryPreventionAnswers(r.Context(), event)
	if err != nil {
		MapDBError(w, err)
		return
	}
	hindsight, err := s.loadInjuryPreventionHindsight(r.Context(), event)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, buildInjuryPrevention(event, answers, hindsight))
}
