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

// エグ練大会。冬(11月〜3月)は大会が少ないので、その回で一番エグかった練習を書いてもらい、
// 運営が選んだトップ3をみんなで投票して1位を決める。
//
// 回は2週間ごとで、開始日(月曜)から日程が決まる。
//   - 1週目の月曜〜2週目の水曜(10日間): 応募。1人1件、100文字まで。書き直せる。
//   - 2週目の木曜: 運営がトップ3を選ぶ。
//   - 金〜日: 投票。1人1票で、締め切りまでは入れ直せる。
//   - 翌週の月曜: 結果を出し、次の回の応募が始まる。
//
// シーズンは11月の最初の月曜に始まり、応募の締め切りが3月31日の終わりを過ぎない回までを開く。
const (
	maxMenuContestBody   = 100
	maxMenuContestPicks  = 3
	menuContestRoundDays = 14
	menuContestEntryDays = 10
	menuContestVoteDay   = 11
)

type menuContestRound struct {
	Start  time.Time // 日本時間の0時
	Number int       // シーズンの何回目か(1から)
}

func (r menuContestRound) entryEndsAt() time.Time { return r.Start.AddDate(0, 0, menuContestEntryDays) }
func (r menuContestRound) voteStartsAt() time.Time {
	return r.Start.AddDate(0, 0, menuContestVoteDay)
}
func (r menuContestRound) voteEndsAt() time.Time { return r.Start.AddDate(0, 0, menuContestRoundDays) }

// key は DB に入れる回の開始日。日本時間の日付を UTC の0時で表す。
func (r menuContestRound) key() time.Time {
	y, m, d := r.Start.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// menuContestSeason は year 年11月に始まるシーズンの回を古い順に返す。
func menuContestSeason(year int) []menuContestRound {
	start := time.Date(year, time.November, 1, 0, 0, 0, 0, jst.Zone)
	for start.Weekday() != time.Monday {
		start = start.AddDate(0, 0, 1)
	}
	end := time.Date(year+1, time.April, 1, 0, 0, 0, 0, jst.Zone)
	var rounds []menuContestRound
	for r := (menuContestRound{Start: start, Number: 1}); !r.entryEndsAt().After(end); r = (menuContestRound{Start: r.Start.AddDate(0, 0, menuContestRoundDays), Number: r.Number + 1}) {
		rounds = append(rounds, r)
	}
	return rounds
}

// currentMenuContestRound は now を含む回(応募から投票の締め切りまで)を返す。シーズン外なら false。
func currentMenuContestRound(now time.Time) (menuContestRound, bool) {
	now = now.In(jst.Zone)
	for _, year := range []int{now.Year() - 1, now.Year()} {
		for _, r := range menuContestSeason(year) {
			if !now.Before(r.Start) && now.Before(r.voteEndsAt()) {
				return r, true
			}
		}
	}
	return menuContestRound{}, false
}

// nextMenuContestStart は now より後に始まる一番近い回の開始時刻。
func nextMenuContestStart(now time.Time) time.Time {
	now = now.In(jst.Zone)
	for _, year := range []int{now.Year() - 1, now.Year(), now.Year() + 1} {
		for _, r := range menuContestSeason(year) {
			if r.Start.After(now) {
				return r.Start
			}
		}
	}
	return time.Time{}
}

// menuContestRoundAt は開始日 key(UTC の0時)の回を返す。
func menuContestRoundAt(key time.Time) (menuContestRound, bool) {
	for _, year := range []int{key.Year() - 1, key.Year()} {
		for _, r := range menuContestSeason(year) {
			if r.key().Equal(key) {
				return r, true
			}
		}
	}
	return menuContestRound{}, false
}

// 回の段階。entry: 応募中 / picking: 運営が選考中 / voting: 投票中
func menuContestPhase(r menuContestRound, now time.Time, hasFinalists bool) string {
	switch {
	case now.Before(r.entryEndsAt()):
		return "entry"
	case !hasFinalists || now.Before(r.voteStartsAt()):
		return "picking"
	default:
		return "voting"
	}
}

type menuContestEntryDTO struct {
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updated_at"`
}

type menuContestFinalistDTO struct {
	ID         uuid.UUID `json:"id"`
	Body       string    `json:"body"`
	AuthorName *string   `json:"author_name"`
	Mine       bool      `json:"mine"`
	// Votes は結果を出すときだけ入れる。投票中は見せない。
	Votes  *int64 `json:"votes,omitempty"`
	Winner bool   `json:"winner,omitempty"`
}

type menuContestRoundDTO struct {
	Start        string                   `json:"start"`
	Number       int                      `json:"number"`
	Phase        string                   `json:"phase"`
	EntryEndsAt  time.Time                `json:"entry_ends_at"`
	VoteStartsAt time.Time                `json:"vote_starts_at"`
	VoteEndsAt   time.Time                `json:"vote_ends_at"`
	MyEntry      *menuContestEntryDTO     `json:"my_entry"`
	Finalists    []menuContestFinalistDTO `json:"finalists"`
	MyVote       *uuid.UUID               `json:"my_vote"`
}

type menuContestResultDTO struct {
	Start     string                   `json:"start"`
	Number    int                      `json:"number"`
	Finalists []menuContestFinalistDTO `json:"finalists"`
}

func menuContestRoundJSON(r menuContestRound, phase string) menuContestRoundDTO {
	return menuContestRoundDTO{
		Start:        r.Start.Format(time.DateOnly),
		Number:       r.Number,
		Phase:        phase,
		EntryEndsAt:  r.entryEndsAt(),
		VoteStartsAt: r.voteStartsAt(),
		VoteEndsAt:   r.voteEndsAt(),
		Finalists:    []menuContestFinalistDTO{},
	}
}

func finalistsJSON(rows []db.ListMenuContestFinalistsRow, viewer uuid.UUID, withVotes bool) []menuContestFinalistDTO {
	var top int64
	for _, row := range rows {
		top = max(top, row.Votes)
	}
	out := make([]menuContestFinalistDTO, 0, len(rows))
	for _, row := range rows {
		f := menuContestFinalistDTO{ID: row.ID, Body: row.Body, Mine: viewer != uuid.Nil && row.AuthorID == viewer}
		if row.AuthorNicknamePublic {
			f.AuthorName = row.AuthorNickname
		}
		if withVotes {
			votes := row.Votes
			f.Votes = &votes
			// 同票なら全員1位。1票も入らなかった回は1位なし。
			f.Winner = top > 0 && row.Votes == top
		}
		out = append(out, f)
	}
	return out
}

func viewerID(r *http.Request) uuid.UUID {
	if identity, ok := userIdentity(r.Context()); ok {
		return identity.UserID
	}
	return uuid.Nil
}

// lastMenuContestResult は投票が締め切られた一番新しい回の結果。
func (s *Server) lastMenuContestResult(ctx context.Context, now time.Time, viewer uuid.UUID) (*menuContestResultDTO, error) {
	// 開始日が「今日の14日前」以前の回は、投票が締め切られている。
	y, m, d := now.In(jst.Zone).AddDate(0, 0, -menuContestRoundDays).Date()
	key, err := s.q.LatestMenuContestResultRound(ctx, time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	round, ok := menuContestRoundAt(key)
	if !ok {
		return nil, nil
	}
	rows, err := s.q.ListMenuContestFinalists(ctx, key)
	if err != nil {
		return nil, err
	}
	return &menuContestResultDTO{
		Start:     round.Start.Format(time.DateOnly),
		Number:    round.Number,
		Finalists: finalistsJSON(rows, viewer, true),
	}, nil
}

// getMenuContest は今の回の日程・自分の応募・トップ3(投票中だけ)と、前の回の結果を返す。ログインなしでも見られる。
func (s *Server) getMenuContest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := s.now()
	viewer := viewerID(r)
	out := map[string]any{"round": nil}

	if round, ok := currentMenuContestRound(now); ok {
		rows, err := s.q.ListMenuContestFinalists(ctx, round.key())
		if err != nil {
			MapDBError(w, err)
			return
		}
		dto := menuContestRoundJSON(round, menuContestPhase(round, now, len(rows) > 0))
		if dto.Phase == "voting" {
			dto.Finalists = finalistsJSON(rows, viewer, false)
		}
		if viewer != uuid.Nil {
			entry, err := s.q.GetMyMenuContestEntry(ctx, db.GetMyMenuContestEntryParams{RoundStart: round.key(), AuthorID: viewer})
			if err == nil {
				dto.MyEntry = &menuContestEntryDTO{Body: entry.Body, UpdatedAt: entry.UpdatedAt}
			} else if !errors.Is(err, pgx.ErrNoRows) {
				MapDBError(w, err)
				return
			}
			if dto.Phase == "voting" {
				vote, err := s.q.GetMyMenuContestVote(ctx, db.GetMyMenuContestVoteParams{RoundStart: round.key(), VoterID: viewer})
				if err == nil {
					dto.MyVote = &vote
				} else if !errors.Is(err, pgx.ErrNoRows) {
					MapDBError(w, err)
					return
				}
			}
		}
		out["round"] = dto
	} else {
		out["next_starts_at"] = nextMenuContestStart(now)
	}

	last, err := s.lastMenuContestResult(ctx, now, viewer)
	if err != nil {
		MapDBError(w, err)
		return
	}
	out["last_result"] = last
	JSON(w, http.StatusOK, out)
}

// entryRoundOrError は応募を受け付けている回を返す。受付外ならエラーを書いて false。
func (s *Server) entryRoundOrError(w http.ResponseWriter) (menuContestRound, bool) {
	now := s.now()
	round, ok := currentMenuContestRound(now)
	if !ok || !now.Before(round.entryEndsAt()) {
		Error(w, http.StatusConflict, "closed", "いまは応募を受け付けていません")
		return menuContestRound{}, false
	}
	return round, true
}

func (s *Server) saveMenuContestEntry(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	body := strings.TrimSpace(req.Body)
	if Body(body, maxMenuContestBody) != "" {
		FieldError(w, map[string]string{"body": "1〜100文字で書いてください"})
		return
	}
	round, ok := s.entryRoundOrError(w)
	if !ok {
		return
	}
	row, err := s.q.UpsertMenuContestEntry(r.Context(), db.UpsertMenuContestEntryParams{
		ID:         uuid.New(),
		RoundStart: round.key(),
		AuthorID:   me.ID,
		Body:       body,
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, menuContestEntryDTO{Body: row.Body, UpdatedAt: row.UpdatedAt})
}

func (s *Server) deleteMenuContestEntry(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	round, ok := s.entryRoundOrError(w)
	if !ok {
		return
	}
	if _, err := s.q.DeleteMyMenuContestEntry(r.Context(), db.DeleteMyMenuContestEntryParams{RoundStart: round.key(), AuthorID: me.ID}); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) voteMenuContest(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		EntryID uuid.UUID `json:"entry_id"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	now := s.now()
	round, ok := currentMenuContestRound(now)
	if !ok {
		Error(w, http.StatusConflict, "closed", "いまは投票を受け付けていません")
		return
	}
	rows, err := s.q.ListMenuContestFinalists(r.Context(), round.key())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if menuContestPhase(round, now, len(rows) > 0) != "voting" {
		Error(w, http.StatusConflict, "closed", "いまは投票を受け付けていません")
		return
	}
	var picked *db.ListMenuContestFinalistsRow
	for i := range rows {
		if rows[i].ID == req.EntryID {
			picked = &rows[i]
		}
	}
	if picked == nil {
		FieldError(w, map[string]string{"entry_id": "トップ3の中から選んでください"})
		return
	}
	if picked.AuthorID == me.ID {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "自分の応募には投票できません")
		return
	}
	if err := s.q.UpsertMenuContestVote(r.Context(), db.UpsertMenuContestVoteParams{RoundStart: round.key(), VoterID: me.ID, EntryID: picked.ID}); err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"my_vote": picked.ID})
}

// adminMenuContest は今の回の応募を全部返す。運営はここから読んでトップ3を選ぶ。
func (s *Server) adminMenuContest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	now := s.now()
	round, ok := currentMenuContestRound(now)
	if !ok {
		JSON(w, http.StatusOK, map[string]any{"round": nil, "entries": []any{}, "next_starts_at": nextMenuContestStart(now)})
		return
	}
	ctx := r.Context()
	finalists, err := s.q.ListMenuContestFinalists(ctx, round.key())
	if err != nil {
		MapDBError(w, err)
		return
	}
	votes, err := s.q.CountMenuContestVotes(ctx, round.key())
	if err != nil {
		MapDBError(w, err)
		return
	}
	rows, err := s.q.ListMenuContestEntries(ctx, round.key())
	if err != nil {
		MapDBError(w, err)
		return
	}
	entries := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, map[string]any{
			"id":          row.ID,
			"body":        row.Body,
			"author_name": row.AuthorName,
			"finalist":    row.Finalist,
			"created_at":  row.CreatedAt,
			"updated_at":  row.UpdatedAt,
		})
	}
	JSON(w, http.StatusOK, map[string]any{
		"round":   menuContestRoundJSON(round, menuContestPhase(round, now, len(finalists) > 0)),
		"entries": entries,
		// 投票が1票でも入ったら、トップ3は変えられない。
		"votes": votes,
		// 選べるのは応募の締め切りから投票の締め切りまで。
		"can_pick": !now.Before(round.entryEndsAt()) && votes == 0,
	})
}

func (s *Server) setMenuContestFinalists(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req struct {
		EntryIDs []uuid.UUID `json:"entry_ids"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	ids := map[uuid.UUID]bool{}
	for _, id := range req.EntryIDs {
		ids[id] = true
	}
	if len(ids) == 0 || len(ids) > maxMenuContestPicks || len(ids) != len(req.EntryIDs) {
		FieldError(w, map[string]string{"entry_ids": "1〜3件選んでください"})
		return
	}
	now := s.now()
	round, ok := currentMenuContestRound(now)
	if !ok || now.Before(round.entryEndsAt()) {
		Error(w, http.StatusConflict, "closed", "応募の締め切り後に選べます")
		return
	}
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	votes, err := q.CountMenuContestVotes(ctx, round.key())
	if err != nil {
		MapDBError(w, err)
		return
	}
	if votes > 0 {
		Error(w, http.StatusConflict, "conflict", "投票が始まっているので、トップ3は変えられません")
		return
	}
	if err := q.DeleteMenuContestFinalists(ctx, round.key()); err != nil {
		MapDBError(w, err)
		return
	}
	for _, id := range req.EntryIDs {
		n, err := q.InsertMenuContestFinalist(ctx, db.InsertMenuContestFinalistParams{EntryID: id, RoundStart: round.key()})
		if err != nil {
			MapDBError(w, err)
			return
		}
		if n == 0 {
			FieldError(w, map[string]string{"entry_ids": "この回の応募から選んでください"})
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		MapDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
