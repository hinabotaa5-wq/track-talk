package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5"
)

type practiceInput struct {
	Venue      string
	StartsAt   time.Time
	Event      string
	EventOther string
	Capacity   int32
}

// 練習相手募集の種目は、アプリ内の種目にリレーを足したもの(コーチの担当種目と同じ)。
func validPracticeEvent(code string) bool {
	return validEvent(code) || code == relayEvent
}

// practiceLabel は種目の表示名。「その他」は、選べた頃に自分で書いた名前(other)を出す。
func practiceLabel(event, other string) string {
	if event == "other" {
		return other
	}
	return eventLabel(event)
}

func practiceTitle(event, other string, starts time.Time) string {
	return practiceLabel(event, other) + " " + starts.In(jst.Zone).Format("1月2日 15:04")
}

func normalizePractice(req postReq) (practiceInput, map[string]string) {
	fields := map[string]string{}
	venue, errMsg := textLimit(req.Venue, 80, "練習場所を入力してください", "練習場所は80文字以内で入力してください", true)
	if errMsg != "" {
		fields["venue"] = errMsg
	}
	var starts time.Time
	rawStart := strings.TrimSpace(req.StartsAt)
	if rawStart == "" {
		fields["starts_at"] = "練習日時を入力してください"
	} else if parsed, err := time.Parse(time.RFC3339, rawStart); err != nil {
		fields["starts_at"] = "練習日時が不正です"
	} else {
		starts = parsed
	}
	event := strings.TrimSpace(req.Event)
	if !validPracticeEvent(event) {
		fields["event"] = "種目を選んでください"
	}
	other := ""
	if event == "other" {
		value, msg := textLimit(req.EventOther, 40, "種目名を入力してください", "種目名は40文字以内で入力してください", true)
		if msg != "" {
			fields["event_other"] = msg
		}
		other = value
	}
	if req.Capacity < 1 || req.Capacity > 99 {
		fields["capacity"] = "募集人数は1〜99で入力してください"
	}
	return practiceInput{
		Venue:      venue,
		StartsAt:   starts,
		Event:      event,
		EventOther: other,
		Capacity:   int32(req.Capacity),
	}, fields
}

func practiceFromDetail(row db.PracticeDetail, accepted int32) *practiceDTO {
	status := "open"
	if row.DissolvedAt != nil {
		status = "dissolved"
	} else if accepted >= row.Capacity {
		status = "full"
	}
	return &practiceDTO{
		Venue:         row.Venue,
		StartsAt:      row.StartsAt,
		Event:         row.Event,
		EventOther:    row.EventOther,
		EventLabel:    practiceLabel(row.Event, row.EventOther),
		Capacity:      row.Capacity,
		DissolvedAt:   row.DissolvedAt,
		AcceptedCount: accepted,
		Status:        status,
		Ratable:       practiceRatable(row),
	}
}

// practiceRatable は、練習が開かれたものとして評価できるか。練習の日時より前に解散したものは中止として扱い、
// 参加者を「ドタキャン」などと評価できないようにする。
func practiceRatable(row db.PracticeDetail) bool {
	return row.DissolvedAt != nil && !row.Canceled
}

func savePractice(ctx context.Context, q *db.Queries, postID uuid.UUID, in practiceInput) error {
	return q.UpsertPracticeDetail(ctx, db.UpsertPracticeDetailParams{
		PostID:     postID,
		Venue:      in.Venue,
		StartsAt:   in.StartsAt,
		Event:      in.Event,
		EventOther: in.EventOther,
		Capacity:   in.Capacity,
	})
}

func (s *Server) createPractice(w http.ResponseWriter, r *http.Request, author db.Profile, params db.CreatePostParams, in practiceInput) {
	if !in.StartsAt.After(time.Now()) {
		FieldError(w, map[string]string{"starts_at": "練習日時は現在より後にしてください"})
		return
	}
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
	if err := savePractice(r.Context(), q, row.ID, in); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, row.ID, http.StatusCreated)
}

func (s *Server) updatePractice(w http.ResponseWriter, r *http.Request, author db.Profile, id uuid.UUID) {
	params, _, in, _, ok := s.parsePostReq(w, r, "practice")
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
	detail, err := q.LockPracticeDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
			return
		}
		MapDBError(w, err)
		return
	}
	if detail.DissolvedAt == nil && !in.StartsAt.After(time.Now()) {
		FieldError(w, map[string]string{"starts_at": "練習日時は現在より後にしてください"})
		return
	}
	accepted, err := q.CountAcceptedApplications(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if in.Capacity < accepted {
		FieldError(w, map[string]string{"capacity": "募集人数は承認済みの人数以上にしてください"})
		return
	}
	if _, err := q.UpdatePost(r.Context(), db.UpdatePostParams{
		ID:       id,
		AuthorID: author.ID,
		Title:    params.Title,
		Body:     params.Body,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusForbidden, "forbidden", "編集できません")
			return
		}
		MapDBError(w, err)
		return
	}
	if err := savePractice(r.Context(), q, id, in); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, id, http.StatusOK)
}

func (s *Server) attachPractice(ctx context.Context, item postDTO) (postDTO, error) {
	if item.Kind != "practice" {
		return item, nil
	}
	detail, err := s.q.GetPracticeDetail(ctx, item.ID)
	if err != nil {
		return item, err
	}
	count, err := s.q.CountAcceptedApplications(ctx, item.ID)
	if err != nil {
		return item, err
	}
	item.Practice = practiceFromDetail(detail, count)
	reputation, err := s.reputationFor(ctx, item.authorID, true)
	if err != nil {
		return item, err
	}
	item.Practice.Reputation = &reputation
	return item, nil
}

func (s *Server) attachPracticeSummaries(ctx context.Context, items []postDTO) error {
	ids := make([]uuid.UUID, 0)
	for _, item := range items {
		if item.Kind == "practice" {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ListPracticeDetails(ctx, ids)
	if err != nil {
		return err
	}
	byID := map[uuid.UUID]db.PracticeDetail{}
	for _, row := range rows {
		byID[row.PostID] = row
	}
	counts, err := s.q.ListAcceptedCounts(ctx, ids)
	if err != nil {
		return err
	}
	accepted := map[uuid.UUID]int32{}
	for _, row := range counts {
		accepted[row.PostID] = row.AcceptedCount
	}
	for i := range items {
		row, ok := byID[items[i].ID]
		if !ok {
			continue
		}
		items[i].Practice = practiceFromDetail(row, accepted[row.PostID])
	}
	if err := s.attachPracticeReputationCounts(ctx, items); err != nil {
		return err
	}
	return nil
}

func (s *Server) attachPracticeReputationCounts(ctx context.Context, items []postDTO) error {
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0)
	for _, item := range items {
		if item.Kind != "practice" || item.Practice == nil || seen[item.authorID] {
			continue
		}
		seen[item.authorID] = true
		ids = append(ids, item.authorID)
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.q.ListPracticeReputationCounts(ctx, ids)
	if err != nil {
		return err
	}
	byUser := map[uuid.UUID]practiceReputationDTO{}
	for _, row := range rows {
		byUser[row.RateeID] = practiceReputationDTO{Good: row.Good, Ok: row.Ok, Bad: row.Bad}
	}
	for i := range items {
		if items[i].Kind != "practice" || items[i].Practice == nil {
			continue
		}
		reputation := byUser[items[i].authorID]
		items[i].Practice.Reputation = &reputation
	}
	return nil
}

func (s *Server) reputationFor(ctx context.Context, userID uuid.UUID, withComments bool) (practiceReputationDTO, error) {
	counts, err := s.q.PracticeReputationCounts(ctx, userID)
	if err != nil {
		return practiceReputationDTO{}, err
	}
	reputation := practiceReputationDTO{Good: counts.Good, Ok: counts.Ok, Bad: counts.Bad}
	if !withComments {
		return reputation, nil
	}
	rows, err := s.q.ListPracticeReputationComments(ctx, userID)
	if err != nil {
		return practiceReputationDTO{}, err
	}
	comments := make([]practiceReputationCommentDTO, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, practiceReputationCommentDTO{
			ID:        row.ID,
			Score:     row.Score,
			Comment:   row.Comment,
			CreatedAt: row.CreatedAt,
		})
	}
	reputation.Comments = comments
	return reputation, nil
}

// hidePracticeRating は、通報された練習の評価のひとことを、練習の投稿から外す。
func (s *Server) hidePracticeRating(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.HidePracticeRating(r.Context(), id)
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

func (s *Server) loadPracticePost(w http.ResponseWriter, r *http.Request) (db.GetPostRow, bool) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return db.GetPostRow{}, false
	}
	row, err := s.q.GetPost(r.Context(), id)
	if err != nil {
		MapDBError(w, err)
		return db.GetPostRow{}, false
	}
	if row.Kind != "practice" {
		Error(w, http.StatusNotFound, "not_found", "見つかりませんでした")
		return db.GetPostRow{}, false
	}
	return row, true
}

func (s *Server) attachPracticeViewer(r *http.Request, item postDTO) (postDTO, error) {
	if item.Kind != "practice" || item.Practice == nil {
		return item, nil
	}
	if uid, ok := UserID(r.Context()); ok {
		ratedIDs, err := s.q.ListRatedApplicationIDs(r.Context(), db.ListRatedApplicationIDsParams{
			PostID:  item.ID,
			RaterID: uid,
		})
		if err != nil {
			return item, err
		}
		rated := map[uuid.UUID]bool{}
		for _, id := range ratedIDs {
			rated[id] = true
		}
		app, err := s.q.GetPracticeApplicationByUser(r.Context(), db.GetPracticeApplicationByUserParams{
			PostID: item.ID,
			UserID: uid,
		})
		if err == nil {
			item.Practice.MyApplication = app.Status
			item.Practice.MyApplicationID = &app.ID
			item.Practice.RatedHost = rated[app.ID]
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return item, err
		}
		if item.Mine {
			rows, err := s.q.ListPracticeApplications(r.Context(), item.ID)
			if err != nil {
				return item, err
			}
			userIDs := make([]uuid.UUID, 0, len(rows))
			for _, row := range rows {
				userIDs = append(userIDs, row.UserID)
			}
			counts, err := s.q.ListApplicantAttendanceCounts(r.Context(), userIDs)
			if err != nil {
				return item, err
			}
			attendance := map[uuid.UUID]practiceAttendanceDTO{}
			for _, c := range counts {
				attendance[c.UserID] = practiceAttendanceDTO{Total: c.Total, NoShow: c.NoShow}
			}
			apps := make([]practiceApplicationDTO, 0, len(rows))
			for _, row := range rows {
				apps = append(apps, practiceApplicationDTO{
					ID:         row.ID,
					Status:     row.Status,
					CreatedAt:  row.CreatedAt,
					Rated:      rated[row.ID],
					Attendance: attendance[row.UserID],
				})
			}
			item.Practice.Applications = apps
		}
	}
	return item, nil
}

func practiceClosed(detail db.PracticeDetail) (int, string) {
	if detail.DissolvedAt != nil {
		return http.StatusConflict, "解散した募集です"
	}
	return 0, ""
}

func (s *Server) practiceCommentAllowed(ctx context.Context, post db.GetPostRow, userID uuid.UUID) (bool, error) {
	if post.Kind != "practice" {
		return true, nil
	}
	detail, err := s.q.GetPracticeDetail(ctx, post.ID)
	if err != nil {
		return false, err
	}
	if detail.DissolvedAt == nil {
		accepted, err := s.q.CountAcceptedApplications(ctx, post.ID)
		if err != nil {
			return false, err
		}
		if accepted < detail.Capacity {
			return true, nil
		}
	}
	if post.AuthorID == userID {
		return true, nil
	}
	app, err := s.q.GetPracticeApplicationByUser(ctx, db.GetPracticeApplicationByUserParams{
		PostID: post.ID,
		UserID: userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return app.Status == "accepted", nil
}

func (s *Server) applyPractice(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	post, ok := s.loadPracticePost(w, r)
	if !ok {
		return
	}
	if post.AuthorID == me.ID {
		Error(w, http.StatusForbidden, "forbidden", "自分の募集には参加できません")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	detail, err := q.LockPracticeDetail(r.Context(), post.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if status, message := practiceClosed(detail); status != 0 {
		Error(w, status, "conflict", message)
		return
	}
	if !detail.StartsAt.After(time.Now()) {
		Error(w, http.StatusConflict, "conflict", "練習の日時を過ぎています")
		return
	}
	accepted, err := q.CountAcceptedApplications(r.Context(), post.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if accepted >= detail.Capacity {
		Error(w, http.StatusConflict, "conflict", "満員です")
		return
	}
	existing, err := q.GetPracticeApplicationByUser(r.Context(), db.GetPracticeApplicationByUserParams{
		PostID: post.ID,
		UserID: me.ID,
	})
	if err == nil {
		switch existing.Status {
		case "accepted":
			Error(w, http.StatusConflict, "conflict", "すでに参加が承認されています")
		case "pending":
			Error(w, http.StatusConflict, "conflict", "すでに参加を申請しています")
		default:
			Error(w, http.StatusConflict, "conflict", "この募集には参加できません")
		}
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	applicationID := uuid.New()
	if err := q.InsertPracticeApplication(r.Context(), db.InsertPracticeApplicationParams{
		ID:     applicationID,
		PostID: post.ID,
		UserID: me.ID,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := insertNotification(r.Context(), q, post.AuthorID, "practice_application", post.ID, nil, &applicationID); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, post.ID, http.StatusCreated)
}

func (s *Server) acceptPractice(w http.ResponseWriter, r *http.Request) {
	s.decidePractice(w, r, "accepted")
}

func (s *Server) declinePractice(w http.ResponseWriter, r *http.Request) {
	s.decidePractice(w, r, "declined")
}

func (s *Server) decidePractice(w http.ResponseWriter, r *http.Request, status string) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	post, ok := s.loadPracticePost(w, r)
	if !ok {
		return
	}
	if post.AuthorID != me.ID {
		Error(w, http.StatusForbidden, "forbidden", "この申請を処理できません")
		return
	}
	applicationID, ok := parseUUIDParam(w, r, "applicationID")
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
	detail, err := q.LockPracticeDetail(r.Context(), post.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if code, message := practiceClosed(detail); code != 0 {
		Error(w, code, "conflict", message)
		return
	}
	application, err := q.GetPracticeApplication(r.Context(), db.GetPracticeApplicationParams{
		ID:     applicationID,
		PostID: post.ID,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if status == "accepted" {
		// 作成・編集と同じく、日時を過ぎた練習には参加を決められない。
		if !detail.StartsAt.After(time.Now()) {
			Error(w, http.StatusConflict, "conflict", "練習の日時を過ぎています")
			return
		}
		accepted, err := q.CountAcceptedApplications(r.Context(), post.ID)
		if err != nil {
			MapDBError(w, err)
			return
		}
		if accepted >= detail.Capacity {
			Error(w, http.StatusConflict, "conflict", "満員です")
			return
		}
	}
	n, err := q.UpdatePracticeApplicationStatus(r.Context(), db.UpdatePracticeApplicationStatusParams{
		ID:     applicationID,
		PostID: post.ID,
		Status: status,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusConflict, "conflict", "この申請は処理済みです")
		return
	}
	if status == "accepted" {
		if err := insertNotification(r.Context(), q, application.UserID, "practice_accepted", post.ID, nil, &application.ID); err != nil {
			MapDBError(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, post.ID, http.StatusOK)
}

func (s *Server) dissolvePractice(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	post, ok := s.loadPracticePost(w, r)
	if !ok {
		return
	}
	if post.AuthorID != me.ID {
		Error(w, http.StatusForbidden, "forbidden", "解散できません")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := s.q.WithTx(tx)
	detail, err := q.LockPracticeDetail(r.Context(), post.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	n, err := q.DissolvePractice(r.Context(), post.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusConflict, "conflict", "解散した募集です")
		return
	}
	accepted, err := q.ListAcceptedPracticeApplications(r.Context(), post.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	// 練習の日時より前の解散は中止なので、評価のお願いは送らない。
	if len(accepted) > 0 && !time.Now().Before(detail.StartsAt) {
		if err := insertNotification(r.Context(), q, post.AuthorID, "practice_rate", post.ID, nil, nil); err != nil {
			MapDBError(w, err)
			return
		}
		for _, application := range accepted {
			if err := insertNotification(r.Context(), q, application.UserID, "practice_rate", post.ID, nil, &application.ID); err != nil {
				MapDBError(w, err)
				return
			}
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, post.ID, http.StatusOK)
}

type ratingReq struct {
	Score   string `json:"score"`
	Comment string `json:"comment"`
}

func normalizeRating(req ratingReq) (string, string, map[string]string) {
	fields := map[string]string{}
	score := strings.TrimSpace(req.Score)
	switch score {
	case "good", "ok", "bad", "no_show":
	default:
		fields["score"] = "評価を選んでください"
	}
	comment, msg := textLimit(req.Comment, 200, "", "コメントは200文字以内で入力してください", false)
	if msg != "" {
		fields["comment"] = msg
	} else if looksLikeContact(comment) {
		// ひとことは練習の投稿で誰でも読めるので、相手や自分の連絡先・SNS を書かせない。
		fields["comment"] = "URL・メールアドレス・SNS の ID・電話番号などは書けません"
	}
	return score, comment, fields
}

func (s *Server) practiceReputation(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	reputation, err := s.reputationFor(r.Context(), me.ID, true)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, reputation)
}

func (s *Server) ratePractice(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req ratingReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	score, comment, fields := normalizeRating(req)
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	post, ok := s.loadPracticePost(w, r)
	if !ok {
		return
	}
	applicationID, ok := parseUUIDParam(w, r, "applicationID")
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
	detail, err := q.LockPracticeDetail(r.Context(), post.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if detail.DissolvedAt == nil {
		Error(w, http.StatusConflict, "conflict", "解散後に評価できます")
		return
	}
	if !practiceRatable(detail) {
		Error(w, http.StatusConflict, "conflict", "練習の日時より前に解散した練習は評価できません")
		return
	}
	application, err := q.GetPracticeApplication(r.Context(), db.GetPracticeApplicationParams{
		ID:     applicationID,
		PostID: post.ID,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if application.Status != "accepted" {
		Error(w, http.StatusForbidden, "forbidden", "承認された相手だけ評価できます")
		return
	}
	var ratee uuid.UUID
	switch {
	case me.ID == post.AuthorID:
		ratee = application.UserID
	case me.ID == application.UserID:
		ratee = post.AuthorID
		if score == "no_show" {
			Error(w, http.StatusForbidden, "forbidden", "ドタキャンは主催者だけが選べます")
			return
		}
	default:
		Error(w, http.StatusForbidden, "forbidden", "この相手は評価できません")
		return
	}
	if _, err := q.GetPracticeRatingByRater(r.Context(), db.GetPracticeRatingByRaterParams{
		ApplicationID: application.ID,
		RaterID:       me.ID,
	}); err == nil {
		Error(w, http.StatusConflict, "conflict", "すでに評価しています")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		MapDBError(w, err)
		return
	}
	if err := q.InsertPracticeRating(r.Context(), db.InsertPracticeRatingParams{
		ID:            uuid.New(),
		PostID:        post.ID,
		ApplicationID: application.ID,
		RaterID:       me.ID,
		RateeID:       ratee,
		Score:         score,
		Comment:       comment,
	}); err != nil {
		MapDBError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		MapDBError(w, err)
		return
	}
	s.writePost(w, r, post.ID, http.StatusCreated)
}
