package httpx

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
	"github.com/jackc/pgx/v5"
)

// 感覚メモは本人だけが読み書きする。投稿とは別のテーブルで、フィードにも公開ページにも出さない。
// 書ける感覚は1日に1つで、書く日は今日か過去の日から選べる。
const (
	maxSenseNoteBody     = 300
	senseNotesPageSize   = 30
	senseNoteCursorSplit = "_"
	// 何日前の日まで選べるか。
	senseNoteMaxDaysBack = 365
)

type senseNoteReq struct {
	Body  string  `json:"body"`
	Event *string `json:"event"`
	// Day は書く日（日本時間、YYYY-MM-DD）。省略すると今日。
	Day *string `json:"day"`
}

type senseNoteDTO struct {
	ID         uuid.UUID `json:"id"`
	Body       string    `json:"body"`
	Event      *string   `json:"event"`
	EventLabel *string   `json:"event_label"`
	// Day はメモの日（日本時間、YYYY-MM-DD）。
	Day       string    `json:"day"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type senseNoteStatsDTO struct {
	Total int64 `json:"total"`
	Days  int   `json:"days"`
	// Streak は、今日か昨日で終わる、メモを書いた日が続いている日数（日本時間）。
	Streak int   `json:"streak"`
	Today  int64 `json:"today"`
}

func senseNoteFrom(id uuid.UUID, body string, event *string, day, createdAt, updatedAt time.Time) senseNoteDTO {
	out := senseNoteDTO{
		ID:        id,
		Body:      body,
		Event:     event,
		Day:       day.Format(time.DateOnly),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if event != nil {
		label := practiceLabel(*event, "その他")
		out.EventLabel = &label
	}
	return out
}

func normalizeSenseNote(req senseNoteReq) (body string, event *string, fields map[string]string) {
	fields = map[string]string{}
	body = strings.TrimSpace(req.Body)
	if e := Body(body, maxSenseNoteBody); e != "" {
		fields["body"] = "1〜300文字で書いてください"
	}
	if req.Event != nil {
		code := strings.TrimSpace(*req.Event)
		if code != "" {
			if !validEvent(code) {
				fields["event"] = "種目を選び直してください"
			} else {
				event = &code
			}
		}
	}
	return body, event, fields
}

// parseSenseNoteDay は、書く日を決める。省略は今日。未来の日と、遠い過去の日は選べない。
func parseSenseNoteDay(v *string, now time.Time) (time.Time, bool) {
	today := dateOnly(now.In(jst.Zone))
	if v == nil || strings.TrimSpace(*v) == "" {
		return today, true
	}
	day, err := time.Parse(time.DateOnly, strings.TrimSpace(*v))
	if err != nil || day.After(today) || day.Before(today.AddDate(0, 0, -senseNoteMaxDaysBack)) {
		return time.Time{}, false
	}
	return day, true
}

// senseNoteStreak は、今日か昨日で終わる連続した日数を数える。days は新しい順で、重なりがないこと。
func senseNoteStreak(days []time.Time, now time.Time) int {
	today := dateOnly(now.In(jst.Zone))
	streak := 0
	var expect time.Time
	for i, day := range days {
		d := dateOnly(day)
		if i == 0 {
			if d.Equal(today) {
				expect = today
			} else if d.Equal(today.AddDate(0, 0, -1)) {
				expect = today.AddDate(0, 0, -1)
			} else {
				return 0
			}
		}
		if !d.Equal(expect) {
			break
		}
		streak++
		expect = expect.AddDate(0, 0, -1)
	}
	return streak
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *Server) senseNoteStats(r *http.Request, authorID uuid.UUID) (senseNoteStatsDTO, error) {
	now := time.Now()
	total, err := s.q.CountSenseNotes(r.Context(), authorID)
	if err != nil {
		return senseNoteStatsDTO{}, err
	}
	today, err := s.q.CountSenseNotesOnDay(r.Context(), db.CountSenseNotesOnDayParams{AuthorID: authorID, Day: dateOnly(now.In(jst.Zone))})
	if err != nil {
		return senseNoteStatsDTO{}, err
	}
	days, err := s.q.ListSenseNoteDays(r.Context(), authorID)
	if err != nil {
		return senseNoteStatsDTO{}, err
	}
	return senseNoteStatsDTO{Total: total, Days: len(days), Streak: senseNoteStreak(days, now), Today: today}, nil
}

func encodeSenseNoteCursor(n senseNoteDTO) string {
	return n.Day + senseNoteCursorSplit + n.CreatedAt.UTC().Format(time.RFC3339Nano) + senseNoteCursorSplit + n.ID.String()
}

func decodeSenseNoteCursor(v string) (day, at *time.Time, id *uuid.UUID, ok bool) {
	parts := strings.Split(v, senseNoteCursorSplit)
	if len(parts) != 3 {
		return nil, nil, nil, false
	}
	d, err := time.Parse(time.DateOnly, parts[0])
	if err != nil {
		return nil, nil, nil, false
	}
	t, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return nil, nil, nil, false
	}
	u, err := uuid.Parse(parts[2])
	if err != nil {
		return nil, nil, nil, false
	}
	return &d, &t, &u, true
}

func (s *Server) listSenseNotes(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	params := db.ListSenseNotesParams{AuthorID: me.ID, PageSize: senseNotesPageSize + 1}
	if c := r.URL.Query().Get("cursor"); c != "" {
		day, at, id, ok := decodeSenseNoteCursor(c)
		if !ok {
			Error(w, http.StatusBadRequest, "validation_error", "cursorが不正です")
			return
		}
		params.BeforeDay, params.BeforeAt, params.BeforeID = day, at, id
	}
	rows, err := s.q.ListSenseNotes(r.Context(), params)
	if err != nil {
		MapDBError(w, err)
		return
	}
	notes := make([]senseNoteDTO, 0, len(rows))
	for _, row := range rows {
		notes = append(notes, senseNoteFrom(row.ID, row.Body, row.Event, row.Day, row.CreatedAt, row.UpdatedAt))
	}
	var next *string
	if len(notes) > senseNotesPageSize {
		notes = notes[:senseNotesPageSize]
		c := encodeSenseNoteCursor(notes[len(notes)-1])
		next = &c
	}
	stats, err := s.senseNoteStats(r, me.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"notes": notes, "next_cursor": next, "stats": stats})
}

func (s *Server) createSenseNote(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req senseNoteReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	body, event, fields := normalizeSenseNote(req)
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	day, ok := parseSenseNoteDay(req.Day, time.Now())
	if !ok {
		FieldError(w, map[string]string{"day": "今日か、過去の日を選んでください"})
		return
	}
	// 同時に送られても1日に1つを超えないよう、作成者ごとのロックの中で数えて保存する。
	errFull := errors.New("sense note of the day exists")
	var row db.CreateSenseNoteRow
	err := s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.LockSenseNoteAuthor(r.Context(), me.ID); err != nil {
			return err
		}
		written, err := q.CountSenseNotesOnDay(r.Context(), db.CountSenseNotesOnDayParams{AuthorID: me.ID, Day: day})
		if err != nil {
			return err
		}
		if written > 0 {
			return errFull
		}
		row, err = q.CreateSenseNote(r.Context(), db.CreateSenseNoteParams{ID: uuid.New(), AuthorID: me.ID, Body: body, Event: event, Day: day})
		return err
	})
	if errors.Is(err, errFull) {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "感覚メモは1日に1つだけ書けます。その日はもう書いています")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	stats, err := s.senseNoteStats(r, me.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, map[string]any{
		"note":  senseNoteFrom(row.ID, row.Body, row.Event, row.Day, row.CreatedAt, row.UpdatedAt),
		"stats": stats,
	})
}

// updateSenseNote は、メモの文と種目を直す。メモの日は変えない。
func (s *Server) updateSenseNote(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req senseNoteReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	body, event, fields := normalizeSenseNote(req)
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	row, err := s.q.UpdateSenseNote(r.Context(), db.UpdateSenseNoteParams{ID: id, AuthorID: me.ID, Body: body, Event: event})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "感覚メモが見つかりません")
			return
		}
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, senseNoteFrom(row.ID, row.Body, row.Event, row.Day, row.CreatedAt, row.UpdatedAt))
}

func (s *Server) deleteSenseNote(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.DeleteSenseNote(r.Context(), db.DeleteSenseNoteParams{ID: id, AuthorID: me.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "感覚メモが見つかりません")
		return
	}
	stats, err := s.senseNoteStats(r, me.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"stats": stats})
}
