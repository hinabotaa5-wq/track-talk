package httpx

import (
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/hinata/track-talk/apps/api/internal/db"
)

// 集計は5人以上そろったものだけ出す。少ないと誰のことかわかってしまうため。
const injuryPublicMinPeople = 5

const injuryPublicNotes = 20

// 復帰の目安を出す段階。安静中・リハビリ中は「着いた」というより始まりなので出さない。
var injuryRecoveryStages = []string{"jogging", "practice", "race"}

type injuryCountDTO struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	People int    `json:"people"`
}

type injuryTallyDTO struct {
	// People は答えた人数。min_people に届くまで Items は空のまま。
	People int              `json:"people"`
	Items  []injuryCountDTO `json:"items"`
}

type injuryRecoveryStageDTO struct {
	Stage      string `json:"stage"`
	StageLabel string `json:"stage_label"`
	People     int    `json:"people"`
	// LowDays と HighDays は真ん中の半分（25〜75パーセンタイル）で、min_people からだけ入れる。
	LowDays  *int `json:"low_days"`
	HighDays *int `json:"high_days"`
}

type injuryRecoveryDTO struct {
	Severity      string                   `json:"severity"`
	SeverityLabel string                   `json:"severity_label"`
	Stages        []injuryRecoveryStageDTO `json:"stages"`
}

type injuryPublicNoteDTO struct {
	// ID は通報の対象を指すもので、段階の記録そのものの id ではない。
	ID         uuid.UUID `json:"id"`
	Note       string    `json:"note"`
	Stage      string    `json:"stage"`
	StageLabel string    `json:"stage_label"`
	Event      string    `json:"event,omitempty"`
	EventLabel string    `json:"event_label,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type injuryPublicGroupDTO struct {
	BodyPart      string                `json:"body_part"`
	BodyPartLabel string                `json:"body_part_label"`
	Members       int64                 `json:"members"`
	MinPeople     int                   `json:"min_people"`
	Recovery      []injuryRecoveryDTO   `json:"recovery"`
	Places        injuryTallyDTO        `json:"places"`
	Activities    injuryTallyDTO        `json:"activities"`
	PreChanges    injuryTallyDTO        `json:"pre_changes"`
	Events        injuryTallyDTO        `json:"events"`
	Actions       injuryTallyDTO        `json:"actions"`
	Notes         []injuryPublicNoteDTO `json:"notes"`
}

// getPublicInjuryGroup は、ログインしているかどうかに関係なく誰でも、検索エンジンも見られるグループのページ。
// 載せるのは集計と、メンバーが公開を選んだ一言だけで、やりとりはグループの中にとどめる。
func (s *Server) getPublicInjuryGroup(w http.ResponseWriter, r *http.Request) {
	part, ok := injuryGroupPart(w, r)
	if !ok {
		return
	}
	counts, err := s.q.CountInjuryGroupMembersByStage(r.Context(), part)
	if err != nil {
		MapDBError(w, err)
		return
	}
	answers, err := s.q.ListInjuryGroupAnswers(r.Context(), part)
	if err != nil {
		MapDBError(w, err)
		return
	}
	changes, err := s.q.ListInjuryGroupStageChanges(r.Context(), part)
	if err != nil {
		MapDBError(w, err)
		return
	}
	notes, err := s.q.ListPublicInjuryGroupNotes(r.Context(), db.ListPublicInjuryGroupNotesParams{BodyPart: part, Limit: injuryPublicNotes})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, buildPublicInjuryGroup(part, counts, answers, changes, notes))
}

func buildPublicInjuryGroup(
	part string,
	counts []db.CountInjuryGroupMembersByStageRow,
	answers []db.ListInjuryGroupAnswersRow,
	changes []db.ListInjuryGroupStageChangesRow,
	notes []db.ListPublicInjuryGroupNotesRow,
) injuryPublicGroupDTO {
	out := injuryPublicGroupDTO{
		BodyPart:      part,
		BodyPartLabel: injuryBodyPartLabel(part),
		MinPeople:     injuryPublicMinPeople,
		Notes:         []injuryPublicNoteDTO{},
	}
	for _, row := range counts {
		out.Members += row.Members
	}

	places, activities, preChanges, events := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, a := range answers {
		if a.Place != nil {
			places[*a.Place]++
		}
		if a.Activity != nil {
			activities[*a.Activity]++
		}
		for _, code := range a.PreChanges {
			preChanges[code]++
		}
		if a.Event != nil {
			events[*a.Event]++
		}
	}
	out.Places = injuryTally(injuryPlaces, places, len(answers))
	out.Activities = injuryTally(injuryActivities, activities, len(answers))
	out.PreChanges = injuryTally(injuryPreChanges, preChanges, len(answers))
	out.Events = injuryTally(injuryEvents, events, len(answers))

	type key struct{ severity, stage string }
	// 意識的にやったことは、1人が何度段階を変えても1回と数える。
	did := map[uuid.UUID]map[string]bool{}
	// 復帰の目安は、その段階に初めて着いたときだけを使う(記録は古い順に来る)。
	firstDays := map[key]map[uuid.UUID]float64{}
	for _, c := range changes {
		if did[c.UserID] == nil {
			did[c.UserID] = map[string]bool{}
		}
		for _, code := range c.Actions {
			did[c.UserID][code] = true
		}
		if c.Severity == nil {
			continue
		}
		k := key{*c.Severity, c.Stage}
		if firstDays[k] == nil {
			firstDays[k] = map[uuid.UUID]float64{}
		}
		if _, seen := firstDays[k][c.UserID]; !seen {
			firstDays[k][c.UserID] = c.Days
		}
	}
	actions := map[string]int{}
	for _, codes := range did {
		for code := range codes {
			actions[code]++
		}
	}
	out.Actions = injuryTally(injuryActions, actions, len(did))

	out.Recovery = make([]injuryRecoveryDTO, 0, len(injurySeverities))
	for _, severity := range injurySeverities {
		item := injuryRecoveryDTO{Severity: severity.Code, SeverityLabel: severity.Label, Stages: []injuryRecoveryStageDTO{}}
		for _, stage := range injuryRecoveryStages {
			byUser := firstDays[key{severity.Code, stage}]
			days := make([]float64, 0, len(byUser))
			for _, d := range byUser {
				days = append(days, d)
			}
			st := injuryRecoveryStageDTO{Stage: stage, StageLabel: injuryStageLabel(stage), People: len(days)}
			if len(days) >= injuryPublicMinPeople {
				low, high := injuryMiddleHalf(days)
				st.LowDays, st.HighDays = &low, &high
			}
			item.Stages = append(item.Stages, st)
		}
		out.Recovery = append(out.Recovery, item)
	}

	for _, n := range notes {
		dto := injuryPublicNoteDTO{ID: n.NoteID, Note: n.Note, Stage: n.Stage, StageLabel: injuryStageLabel(n.Stage), CreatedAt: n.CreatedAt}
		if n.Event != nil {
			dto.Event = *n.Event
			dto.EventLabel = injuryOptionLabel(injuryEvents, *n.Event)
		}
		out.Notes = append(out.Notes, dto)
	}
	return out
}

// injuryTally は、少なくとも min 人が答えたら、選ばれた選択肢を多い順に並べる。
func injuryTally(options []injuryOption, counts map[string]int, people int) injuryTallyDTO {
	out := injuryTallyDTO{People: people, Items: []injuryCountDTO{}}
	if people < injuryPublicMinPeople {
		return out
	}
	for _, item := range options {
		if counts[item.Code] > 0 {
			out.Items = append(out.Items, injuryCountDTO{Code: item.Code, Label: item.Label, People: counts[item.Code]})
		}
	}
	sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].People > out.Items[j].People })
	return out
}

// injuryMiddleHalf は、25パーセンタイルと75パーセンタイルを日単位で返す。分かっているのはケガをした月だけで、
// 1日として保存しているので、ケガはその月の真ん中にしたものとみなす。
func injuryMiddleHalf(days []float64) (int, int) {
	sorted := append([]float64(nil), days...)
	sort.Float64s(sorted)
	at := func(p float64) int {
		v := sorted[int(math.Round(p*float64(len(sorted)-1)))] - 14
		return int(math.Max(0, math.Round(v)))
	}
	return at(0.25), at(0.75)
}

// hideInjuryNote は、通報された一言を公開ページから外す。段階の変更そのものは集計に数えたままにする。
func (s *Server) hideInjuryNote(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.HideInjuryNote(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
