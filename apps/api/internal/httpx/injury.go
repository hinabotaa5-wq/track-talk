package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

type injuryInput struct {
	BodyPart string
	// GroupStage は、投稿したときの、その部位のグループでの書いた人の段階。グループの外で書いた投稿では nil。
	GroupStage *string
}

type injuryDTO struct {
	BodyPart      string `json:"body_part"`
	BodyPartLabel string `json:"body_part_label"`
	// Stage は、投稿で報告された最新の回復の段階。最初の報告までは空。
	Stage      string            `json:"stage,omitempty"`
	StageLabel string            `json:"stage_label,omitempty"`
	Updates    []injuryUpdateDTO `json:"updates,omitempty"`
}

type injuryUpdateDTO struct {
	ID         uuid.UUID `json:"id"`
	Stage      string    `json:"stage"`
	StageLabel string    `json:"stage_label"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

const maxInjuryUpdates = 50

var injuryStages = []struct{ Code, Label string }{
	{"resting", "安静中"},
	{"rehab", "リハビリ中"},
	{"jogging", "ジョグ再開"},
	{"half", "50%を出せる"},
	{"full", "全力を出せる"},
	{"race", "大会に復帰した"},
}

func injuryStageLabel(code string) string {
	for _, item := range injuryStages {
		if item.Code == code {
			return item.Label
		}
	}
	return code
}

func validInjuryStage(code string) bool {
	return injuryStageLabel(code) != code
}

func injuryUpdateFromRow(row db.InjuryUpdate) injuryUpdateDTO {
	return injuryUpdateDTO{
		ID:         row.ID,
		Stage:      row.Stage,
		StageLabel: injuryStageLabel(row.Stage),
		Body:       row.Body,
		CreatedAt:  row.CreatedAt,
	}
}

type injuryUpdateReq struct {
	Stage string `json:"stage"`
	Body  string `json:"body"`
}

// injuryBodyParts は、ケガのグループ。API と DB では名前を「部位（body part）」のままにしているが、
// 今は各グループが1つの診断名で、当てはまらないものは "other" に入る。並びは発症が多い順(その他は最後)。
var injuryBodyParts = []struct{ Code, Label string }{
	{"hamstring-strain", "ハムストリングの肉離れ"},
	{"sprain", "足首の捻挫"},
	{"shin-splints", "シンスプリント"},
	{"stress-fracture", "疲労骨折"},
	{"calf-strain", "ふくらはぎの肉離れ"},
	{"jumpers-knee", "ジャンパー膝(膝蓋腱炎)"},
	{"achilles-tendinitis", "アキレス腱炎"},
	{"lower-back", "腰の痛み"},
	{"quad-strain", "太もも前の肉離れ"},
	{"shoulder", "肩の痛み"},
	{"elbow", "肘の痛み"},
	{"other", "その他"},
}

func injuryBodyPartLabel(code string) string {
	for _, item := range injuryBodyParts {
		if item.Code == code {
			return item.Label
		}
	}
	return code
}

func validInjuryBodyPart(code string) bool {
	return injuryBodyPartLabel(code) != code
}

func normalizeInjury(req postReq) (injuryInput, map[string]string) {
	fields := map[string]string{}
	part := strings.TrimSpace(req.BodyPart)
	if !validInjuryBodyPart(part) {
		fields["body_part"] = "ケガを選んでください"
	}
	return injuryInput{BodyPart: part}, fields
}

func injuryFromDetail(row db.InjuryDetail) *injuryDTO {
	return &injuryDTO{
		BodyPart:      row.BodyPart,
		BodyPartLabel: injuryBodyPartLabel(row.BodyPart),
	}
}

func saveInjury(ctx context.Context, q *db.Queries, postID uuid.UUID, in injuryInput) error {
	return q.UpsertInjuryDetail(ctx, db.UpsertInjuryDetailParams{
		PostID:     postID,
		BodyPart:   in.BodyPart,
		GroupStage: in.GroupStage,
	})
}

func (s *Server) createInjury(w http.ResponseWriter, r *http.Request, author db.Profile, params db.CreatePostParams, in injuryInput) {
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	params.ID = uuid.New()
	params.AuthorID = author.ID
	row, err := q.CreatePost(r.Context(), params)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if err := saveInjury(r.Context(), q, row.ID, in); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, row.ID, http.StatusCreated)
}

func (s *Server) updateInjury(w http.ResponseWriter, r *http.Request, author db.Profile, id uuid.UUID) {
	params, _, _, in, ok := s.parsePostReqTitle(w, r, "injury", true)
	if !ok {
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	_, err = q.UpdatePost(r.Context(), db.UpdatePostParams{
		ID:       id,
		AuthorID: author.ID,
		Title:    params.Title,
		Body:     params.Body,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusForbidden, "forbidden", "編集できません")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 部位は参加しているグループで決まるので、編集では変えられない。
	current, err := q.GetInjuryDetail(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if current.BodyPart != in.BodyPart {
		FieldError(w, map[string]string{"body_part": "部位は変えられません"})
		return
	}
	if err := saveInjury(r.Context(), q, id, in); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, id, http.StatusOK)
}

func (s *Server) attachInjury(ctx context.Context, item postDTO) (postDTO, error) {
	if item.Kind != "injury" {
		return item, nil
	}
	detail, err := s.q.GetInjuryDetail(ctx, item.ID)
	if err != nil {
		return item, err
	}
	info := injuryFromDetail(detail)
	updates, err := s.q.ListInjuryUpdates(ctx, item.ID)
	if err != nil {
		return item, err
	}
	for _, row := range updates {
		info.Updates = append(info.Updates, injuryUpdateFromRow(row))
	}
	if n := len(updates); n > 0 {
		info.Stage = updates[n-1].Stage
		info.StageLabel = injuryStageLabel(info.Stage)
	}
	item.Injury = info
	return item, nil
}

func (s *Server) attachInjurySummaries(ctx context.Context, items []postDTO) error {
	ids := make([]uuid.UUID, 0)
	for _, item := range items {
		if item.Kind == "injury" {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ListInjuryDetails(ctx, ids)
	if err != nil {
		return err
	}
	stages, err := s.q.ListLatestInjuryStages(ctx, ids)
	if err != nil {
		return err
	}
	stageByID := map[uuid.UUID]string{}
	for _, row := range stages {
		stageByID[row.PostID] = row.Stage
	}
	byID := map[uuid.UUID]db.InjuryDetail{}
	for _, row := range rows {
		byID[row.PostID] = row
	}
	for i := range items {
		if row, ok := byID[items[i].ID]; ok {
			info := injuryFromDetail(row)
			if stage, ok := stageByID[row.PostID]; ok {
				info.Stage = stage
				info.StageLabel = injuryStageLabel(stage)
			}
			items[i].Injury = info
		}
	}
	return nil
}

// injuryOwnedPost は、投稿を読み込み、それが呼び出した本人の書いたケガの投稿かを返す。
func (s *Server) injuryOwnedPost(w http.ResponseWriter, r *http.Request, author db.Profile) (uuid.UUID, bool) {
	postID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return uuid.Nil, false
	}
	post, err := s.q.GetPost(r.Context(), postID)
	if err != nil {
		MapDBError(w, err)
		return uuid.Nil, false
	}
	if post.Kind != "injury" {
		Error(w, http.StatusNotFound, "not_found", "ケガの投稿ではありません")
		return uuid.Nil, false
	}
	if post.AuthorID != author.ID {
		Error(w, http.StatusForbidden, "forbidden", "経過を追加できるのは投稿者だけです")
		return uuid.Nil, false
	}
	return postID, true
}

func normalizeInjuryUpdate(req injuryUpdateReq) (stage, body string, fields map[string]string) {
	fields = map[string]string{}
	stage = strings.TrimSpace(req.Stage)
	if !validInjuryStage(stage) {
		fields["stage"] = "いまの状況を選んでください"
	}
	if e := Body(req.Body, 2000); e != "" {
		fields["body"] = e
	}
	return stage, strings.TrimSpace(req.Body), fields
}

func (s *Server) createInjuryUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	postID, ok := s.injuryOwnedPost(w, r, p)
	if !ok {
		return
	}
	var req injuryUpdateReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	stage, body, fields := normalizeInjuryUpdate(req)
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	// 同時に送られても50件を超えないよう、投稿の行をロックしてから数えて保存する。
	errFull := errors.New("injury updates full")
	var row db.InjuryUpdate
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockInjuryPost(r.Context(), postID); err != nil {
			return err
		}
		count, err := q.CountInjuryUpdates(r.Context(), postID)
		if err != nil {
			return err
		}
		if count >= maxInjuryUpdates {
			return errFull
		}
		row, err = q.CreateInjuryUpdate(r.Context(), db.CreateInjuryUpdateParams{
			ID:     uuid.New(),
			PostID: postID,
			Stage:  stage,
			Body:   body,
		})
		return err
	})
	if errors.Is(err, errFull) {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "経過報告は1つの投稿につき50件までです")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 通知は補足なので、失敗しても経過報告そのものは成功として返す。
	pushTo, err := s.q.NotifyInjuryFollowers(r.Context(), db.NotifyInjuryFollowersParams{PostID: postID, AuthorID: p.ID})
	if err != nil {
		slog.Error("injury update notification failed", "err", err, "post_id", postID.String())
	}
	// 「わかる」や保存をした人が多くても報告した人を待たせないよう、プッシュは裏で1人ずつ送る。
	if len(pushTo) > 0 {
		ctx := context.WithoutCancel(r.Context())
		go func() {
			msg := postPushMessage("復帰の経過が更新されました", postID)
			for _, id := range pushTo {
				s.pushQuietly(ctx, id, notifyInjury, msg, 24*time.Hour)
			}
		}()
	}
	JSON(w, http.StatusCreated, injuryUpdateFromRow(row))
}

func (s *Server) deleteInjuryUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	postID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	updateID, ok := parseUUIDParam(w, r, "updateID")
	if !ok {
		return
	}
	n, err := s.q.DeleteInjuryUpdate(r.Context(), db.DeleteInjuryUpdateParams{ID: updateID, PostID: postID, AuthorID: p.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "経過報告が見つかりません")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateInjuryUpdate は、書いた本人が報告の段階と文を直せるようにする。報告の時刻は元のままで、通知も送らないので、
// 直してもタイムラインの順番は変わらず、フォローしている人に知らせも届かない。
func (s *Server) updateInjuryUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	postID, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	updateID, ok := parseUUIDParam(w, r, "updateID")
	if !ok {
		return
	}
	var req injuryUpdateReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	stage, body, fields := normalizeInjuryUpdate(req)
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	row, err := s.q.UpdateInjuryUpdate(r.Context(), db.UpdateInjuryUpdateParams{
		ID:       updateID,
		PostID:   postID,
		AuthorID: p.ID,
		Stage:    stage,
		Body:     body,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "経過報告が見つかりません")
			return
		}
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, injuryUpdateFromRow(row))
}

// getReadablePost は、読んだり反応したりするリクエストのために投稿を読み込む。ケガの投稿を読めるのは、
// 書いた本人、その部位のグループのメンバー、運営だけで、それ以外の人は断る。
func (s *Server) getReadablePost(w http.ResponseWriter, r *http.Request, id uuid.UUID) (db.GetPostRow, bool) {
	post, err := s.q.GetPost(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return db.GetPostRow{}, false
	}
	if post.Kind != "injury" {
		return post, true
	}
	uid, ok := UserID(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "ログインすると読めます")
		return db.GetPostRow{}, false
	}
	if uid == post.AuthorID {
		return post, true
	}
	admin, err := s.q.IsProfileAdmin(r.Context(), uid)
	if err != nil {
		MapDBError(w, err)
		return db.GetPostRow{}, false
	}
	if admin {
		return post, true
	}
	detail, err := s.q.GetInjuryDetail(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return db.GetPostRow{}, false
	}
	_, err = s.q.GetInjuryGroupMember(r.Context(), db.GetInjuryGroupMemberParams{UserID: uid, BodyPart: detail.BodyPart})
	if errors.Is(err, pgx.ErrNoRows) {
		Error(w, http.StatusForbidden, "not_joined", "グループに参加すると読めます")
		return db.GetPostRow{}, false
	}
	if err != nil {
		MapDBError(w, err)
		return db.GetPostRow{}, false
	}
	return post, true
}
