package httpx

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// injuryGroupFaces は、ロードマップの段階ごとにニックネームを出すメンバーの数。
const injuryGroupFaces = 5

type injuryGroupMemberDTO struct {
	Stage      string `json:"stage"`
	StageLabel string `json:"stage_label"`
	// InjuredMonth は "YYYY-MM"。ケガをした時期は大まかにしか聞かない。
	InjuredMonth string `json:"injured_month"`
	// 質問が選択式になる前に入ったメンバーは、下の答えが空になっている。
	Diagnosis     string   `json:"diagnosis,omitempty"`
	Severity      string   `json:"severity,omitempty"`
	SeverityLabel string   `json:"severity_label,omitempty"`
	SawDoctor     *bool    `json:"saw_doctor,omitempty"`
	Place         string   `json:"place,omitempty"`
	PlaceLabel    string   `json:"place_label,omitempty"`
	Activity      string   `json:"activity,omitempty"`
	ActivityLabel string   `json:"activity_label,omitempty"`
	PreChanges    []string `json:"pre_changes"`
	Event         string   `json:"event,omitempty"`
	EventLabel    string   `json:"event_label,omitempty"`
}

type injuryGroupSummaryDTO struct {
	BodyPart      string                `json:"body_part"`
	BodyPartLabel string                `json:"body_part_label"`
	Members       int64                 `json:"members"`
	Me            *injuryGroupMemberDTO `json:"me,omitempty"`
}

type injuryGroupStageDTO struct {
	Stage      string   `json:"stage"`
	StageLabel string   `json:"stage_label"`
	Members    int64    `json:"members"`
	Faces      []string `json:"faces"`
}

type injuryGroupDTO struct {
	BodyPart      string                `json:"body_part"`
	BodyPartLabel string                `json:"body_part_label"`
	Members       int64                 `json:"members"`
	Stages        []injuryGroupStageDTO `json:"stages"`
	Me            injuryGroupMemberDTO  `json:"me"`
}

func injuryGroupMemberFromRow(row db.InjuryGroupMember) injuryGroupMemberDTO {
	dto := injuryGroupMemberDTO{
		Stage:        row.Stage,
		StageLabel:   injuryStageLabel(row.Stage),
		InjuredMonth: row.InjuredOn.Format("2006-01"),
		SawDoctor:    row.SawDoctor,
		PreChanges:   row.PreChanges,
	}
	if dto.PreChanges == nil {
		dto.PreChanges = []string{}
	}
	if row.Diagnosis != nil {
		dto.Diagnosis = *row.Diagnosis
	}
	if row.Severity != nil {
		dto.Severity, dto.SeverityLabel = *row.Severity, injuryOptionLabel(injurySeverities, *row.Severity)
	}
	if row.Place != nil {
		dto.Place, dto.PlaceLabel = *row.Place, injuryOptionLabel(injuryPlaces, *row.Place)
	}
	if row.Activity != nil {
		dto.Activity, dto.ActivityLabel = *row.Activity, injuryOptionLabel(injuryActivities, *row.Activity)
	}
	if row.Event != nil {
		dto.Event, dto.EventLabel = *row.Event, injuryOptionLabel(injuryEvents, *row.Event)
	}
	return dto
}

// injuryGroupPart は、パスの {part} を読み、知らない部位なら断る。
func injuryGroupPart(w http.ResponseWriter, r *http.Request) (string, bool) {
	part := chi.URLParam(r, "part")
	if !validInjuryBodyPart(part) {
		Error(w, http.StatusNotFound, "not_found", "そのケガのグループはありません")
		return "", false
	}
	return part, true
}

// injuryGroupMember は、その部位のグループでの呼び出した本人のメンバー情報を読み込む。入っていなければ 403 を返す。
func (s *Server) injuryGroupMember(w http.ResponseWriter, r *http.Request) (db.Profile, string, db.InjuryGroupMember, bool) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return db.Profile{}, "", db.InjuryGroupMember{}, false
	}
	part, ok := injuryGroupPart(w, r)
	if !ok {
		return db.Profile{}, "", db.InjuryGroupMember{}, false
	}
	member, err := s.q.GetInjuryGroupMember(r.Context(), db.GetInjuryGroupMemberParams{UserID: p.ID, BodyPart: part})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusForbidden, "not_joined", "グループに参加すると読めます")
		return db.Profile{}, "", db.InjuryGroupMember{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.Profile{}, "", db.InjuryGroupMember{}, false
	}
	return p, part, member, true
}

func (s *Server) listInjuryGroups(w http.ResponseWriter, r *http.Request) {
	counts, err := s.q.CountInjuryGroupMembers(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	members := make(map[string]int64, len(counts))
	for _, row := range counts {
		members[row.BodyPart] = row.Members
	}
	mine := map[string]db.InjuryGroupMember{}
	if uid, ok := UserID(r.Context()); ok {
		rows, err := s.q.ListMyInjuryGroupMembers(r.Context(), uid)
		if err != nil {
			MapDBError(w, err)
			return
		}
		for _, row := range rows {
			mine[row.BodyPart] = row
		}
	}
	groups := make([]injuryGroupSummaryDTO, 0, len(injuryBodyParts))
	for _, part := range injuryBodyParts {
		item := injuryGroupSummaryDTO{BodyPart: part.Code, BodyPartLabel: part.Label, Members: members[part.Code]}
		if row, ok := mine[part.Code]; ok {
			dto := injuryGroupMemberFromRow(row)
			item.Me = &dto
		}
		groups = append(groups, item)
	}
	JSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (s *Server) getInjuryGroup(w http.ResponseWriter, r *http.Request) {
	_, part, member, ok := s.injuryGroupMember(w, r)
	if !ok {
		return
	}
	counts, err := s.q.CountInjuryGroupMembersByStage(r.Context(), part)
	if err != nil {
		MapDBError(w, err)
		return
	}
	faces, err := s.q.ListInjuryGroupFaces(r.Context(), db.ListInjuryGroupFacesParams{BodyPart: part, LimitN: injuryGroupFaces})
	if err != nil {
		MapDBError(w, err)
		return
	}
	byStage := make(map[string]int64, len(counts))
	var total int64
	for _, row := range counts {
		byStage[row.Stage] = row.Members
		total += row.Members
	}
	facesByStage := map[string][]string{}
	for _, row := range faces {
		facesByStage[row.Stage] = append(facesByStage[row.Stage], row.Nickname)
	}
	out := injuryGroupDTO{
		BodyPart:      part,
		BodyPartLabel: injuryBodyPartLabel(part),
		Members:       total,
		Stages:        make([]injuryGroupStageDTO, 0, len(injuryStages)),
		Me:            injuryGroupMemberFromRow(member),
	}
	for _, stage := range injuryStages {
		f := facesByStage[stage.Code]
		if f == nil {
			f = []string{}
		}
		out.Stages = append(out.Stages, injuryGroupStageDTO{
			Stage:      stage.Code,
			StageLabel: stage.Label,
			Members:    byStage[stage.Code],
			Faces:      f,
		})
	}
	JSON(w, http.StatusOK, out)
}

func (s *Server) joinInjuryGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	part, ok := injuryGroupPart(w, r)
	if !ok {
		return
	}
	var req injuryJoinReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	in, fields := normalizeInjuryJoin(part, req, time.Now())
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	row, err := q.UpsertInjuryGroupMember(r.Context(), db.UpsertInjuryGroupMemberParams{
		UserID:     p.ID,
		BodyPart:   part,
		InjuredOn:  in.InjuredOn,
		Diagnosis:  in.Diagnosis,
		Severity:   &in.Severity,
		SawDoctor:  &in.SawDoctor,
		Place:      &in.Place,
		Activity:   &in.Activity,
		PreChanges: in.PreChanges,
		Event:      &in.Event,
		Stage:      in.Stage,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.AddInjuryGroupStageLog(r.Context(), db.AddInjuryGroupStageLogParams{UserID: p.ID, BodyPart: part, Stage: in.Stage, Actions: []string{}}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, injuryGroupMemberFromRow(row))
}

// updateInjuryGroupStage は、メンバーが前にも後ろにも（再発や2回目の手術で）好きな段階へ移れるようにする。
// 離れる段階のあいだに意識してやったことと、次の人への一言（任意）を残す。
// すでに書いた投稿は、書いたときの段階のままにする。
func (s *Server) updateInjuryGroupStage(w http.ResponseWriter, r *http.Request) {
	p, part, member, ok := s.injuryGroupMember(w, r)
	if !ok {
		return
	}
	var req injuryGroupStageReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	in, fields := normalizeInjuryStageChange(req)
	if len(fields) == 0 && in.Stage == member.Stage {
		fields["stage"] = "いまと同じ段階です。変わった段階を選んでください"
	}
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	stage := in.Stage
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	row, err := q.SetInjuryGroupStage(r.Context(), db.SetInjuryGroupStageParams{UserID: p.ID, BodyPart: part, Stage: stage})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.AddInjuryGroupStageLog(r.Context(), db.AddInjuryGroupStageLogParams{
		UserID: p.ID, BodyPart: part, Stage: stage, Actions: in.Actions, Note: in.Note, NotePublic: in.NotePublic,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, injuryGroupMemberFromRow(row))
}

// listInjuryGroupPosts は、グループの投稿を並べる。段階で絞ることもできる。メンバーはどの段階も読める。
func (s *Server) listInjuryGroupPosts(w http.ResponseWriter, r *http.Request) {
	_, part, _, ok := s.injuryGroupMember(w, r)
	if !ok {
		return
	}
	qs := r.URL.Query()
	qs.Set("part", part)
	r.URL.RawQuery = qs.Encode()
	kind := "injury"
	s.listPostsFiltered(w, r, "new", nil, &kind)
}

// createInjuryGroupPost は、メンバーの今の段階でグループに投稿する。ほかの段階は読むだけ。
func (s *Server) createInjuryGroupPost(w http.ResponseWriter, r *http.Request) {
	p, part, member, ok := s.injuryGroupMember(w, r)
	if !ok {
		return
	}
	params, _, _, in, ok := s.parsePostReq(w, r, "injury")
	if !ok {
		return
	}
	if in.BodyPart != part {
		FieldError(w, map[string]string{"body_part": "グループのケガと投稿のケガが合っていません"})
		return
	}
	in.GroupStage = &member.Stage
	s.createInjury(w, r, p, params, in)
}

// leaveInjuryGroup は、呼び出した本人をグループから抜けさせる。段階の履歴はメンバー情報といっしょに消える。
// 書いた投稿は残り、1つずつ削除できる。あとでまた入り直せる。
func (s *Server) leaveInjuryGroup(w http.ResponseWriter, r *http.Request) {
	p, part, _, ok := s.injuryGroupMember(w, r)
	if !ok {
		return
	}
	if _, err := s.q.DeleteInjuryGroupMember(r.Context(), db.DeleteInjuryGroupMemberParams{UserID: p.ID, BodyPart: part}); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
