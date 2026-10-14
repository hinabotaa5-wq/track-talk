package httpx

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// injuryGroupFaces is how many members' nicknames each stage shows on the roadmap.
const injuryGroupFaces = 5

type injuryGroupMemberDTO struct {
	Stage      string `json:"stage"`
	StageLabel string `json:"stage_label"`
	InjuredOn  string `json:"injured_on"`
	Place      string `json:"place"`
	Situation  string `json:"situation"`
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

type injuryJoinReq struct {
	InjuredOn string `json:"injured_on"`
	Place     string `json:"place"`
	Situation string `json:"situation"`
	Stage     string `json:"stage"`
}

type injuryGroupStageReq struct {
	Stage string `json:"stage"`
}

func injuryGroupMemberFromRow(row db.InjuryGroupMember) injuryGroupMemberDTO {
	return injuryGroupMemberDTO{
		Stage:      row.Stage,
		StageLabel: injuryStageLabel(row.Stage),
		InjuredOn:  row.InjuredOn.Format("2006-01-02"),
		Place:      row.Place,
		Situation:  row.Situation,
	}
}

// injuryGroupPart reads the {part} path parameter and rejects an unknown body part.
func injuryGroupPart(w http.ResponseWriter, r *http.Request) (string, bool) {
	part := chi.URLParam(r, "part")
	if !validInjuryBodyPart(part) {
		Error(w, http.StatusNotFound, "not_found", "その部位のグループはありません")
		return "", false
	}
	return part, true
}

// injuryGroupMember loads the caller's membership in the part's group, answering 403 when they have not joined.
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

func normalizeInjuryJoin(req injuryJoinReq) (injuredOn time.Time, place, situation, stage string, fields map[string]string) {
	fields = map[string]string{}
	day, err := time.Parse("2006-01-02", strings.TrimSpace(req.InjuredOn))
	switch {
	case err != nil:
		fields["injured_on"] = "いつ痛めたかを選んでください"
	case day.After(time.Now().AddDate(0, 0, 1)):
		fields["injured_on"] = "未来の日付は選べません"
	default:
		injuredOn = day
	}
	place = strings.TrimSpace(req.Place)
	if n := utf8.RuneCountInString(place); n < 1 || n > 100 {
		fields["place"] = "どこで痛めたかを100文字以内で入力してください"
	}
	situation = strings.TrimSpace(req.Situation)
	if n := utf8.RuneCountInString(situation); n < 1 || n > 500 {
		fields["situation"] = "どんな状況だったかを500文字以内で入力してください"
	}
	stage = strings.TrimSpace(req.Stage)
	if !validInjuryStage(stage) {
		fields["stage"] = "いまの状況を選んでください"
	}
	return injuredOn, place, situation, stage, fields
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
	injuredOn, place, situation, stage, fields := normalizeInjuryJoin(req)
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
		UserID: p.ID, BodyPart: part, InjuredOn: injuredOn, Place: place, Situation: situation, Stage: stage,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := q.AddInjuryGroupStageLog(r.Context(), db.AddInjuryGroupStageLogParams{UserID: p.ID, BodyPart: part, Stage: stage}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, injuryGroupMemberFromRow(row))
}

// updateInjuryGroupStage lets a member move to any stage, forward or back (a relapse, a second surgery).
// Posts already written keep the stage they were written in.
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
	stage := strings.TrimSpace(req.Stage)
	if !validInjuryStage(stage) {
		FieldError(w, map[string]string{"stage": "いまの状況を選んでください"})
		return
	}
	if stage == member.Stage {
		JSON(w, http.StatusOK, injuryGroupMemberFromRow(member))
		return
	}
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
	if err := q.AddInjuryGroupStageLog(r.Context(), db.AddInjuryGroupStageLogParams{UserID: p.ID, BodyPart: part, Stage: stage}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, injuryGroupMemberFromRow(row))
}

// listInjuryGroupPosts lists the group's posts, optionally for one stage. Any member can read every stage.
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

// createInjuryGroupPost writes a post into the group at the member's current stage; other stages are read-only.
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
		FieldError(w, map[string]string{"body_part": "グループの部位と投稿の部位が合っていません"})
		return
	}
	in.GroupStage = &member.Stage
	s.createInjury(w, r, p, params, in)
}

// leaveInjuryGroup takes the caller out of the group. Their stage history goes with the membership;
// the posts they wrote stay, and they can delete those one by one. They can join again later.
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
