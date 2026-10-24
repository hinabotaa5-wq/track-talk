package httpx

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/hinata/track-talk/apps/api/internal/jst"
)

const (
	// 1度に貼り付けられるタイムテーブルの行数。
	maxMeetSessionRows = 300
	// 最後の種目が始まってから「実施中」と見なす長さ(次の種目が無いので終わりが分からない)。
	lastSessionRunning = 3 * time.Hour
)

var meetSessionLayouts = []string{"2006-01-02 15:04", "2006/1/2 15:04", "2006-1-2 15:04"}

type meetSessionInput struct {
	event    string
	round    string
	gender   string
	startsAt time.Time
}

// sessionLabel は「女子100m予選」のように、性別・種目・ラウンドをつなげた表示。
func sessionLabel(gender, event, round string) string {
	return gender + event + round
}

func normalizeGender(v string) (string, bool) {
	switch strings.TrimSpace(v) {
	case "":
		return "", true
	case "男子", "男":
		return "男子", true
	case "女子", "女":
		return "女子", true
	case "混合":
		return "混合", true
	}
	return "", false
}

// parseMeetSessionsCSV は、運営が貼り付けたタイムテーブルを読む。
// 列は「開始日時(日本時間 2026-10-12 10:30),種目,ラウンド,性別」で、ラウンドと性別は空でよい。
// 区切りはカンマかタブ(表計算からの貼り付け)。1行目が日時で読めず、数字も含まないときだけ見出しとして読み飛ばす
// （数字を含むのに読めないのは、日時の書き間違いなのでエラーにする）。
func parseMeetSessionsCSV(text string) ([]meetSessionInput, map[string]string) {
	var rows []meetSessionInput
	errs := map[string]string{}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	first := true
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sep := ","
		if strings.Contains(line, "\t") {
			sep = "\t"
		}
		cols := strings.Split(line, sep)
		for j := range cols {
			cols[j] = strings.TrimSpace(cols[j])
		}
		at, ok := parseMeetSessionTime(cols[0])
		if !ok {
			if first && !strings.ContainsAny(cols[0], "0123456789０１２３４５６７８９") {
				first = false
				continue
			}
			first = false
			errs[lineKey(i)] = "開始日時は 2026-10-12 10:30 の形で入れてください"
			continue
		}
		first = false
		if len(cols) < 2 || cols[1] == "" {
			errs[lineKey(i)] = "種目を入れてください"
			continue
		}
		in := meetSessionInput{startsAt: at, event: cols[1]}
		if len(cols) > 2 {
			in.round = cols[2]
		}
		if len(cols) > 3 {
			g, ok := normalizeGender(cols[3])
			if !ok {
				errs[lineKey(i)] = "性別は 男子・女子・混合 のどれかにしてください"
				continue
			}
			in.gender = g
		}
		if utf8.RuneCountInString(in.event) > 30 || utf8.RuneCountInString(in.round) > 20 {
			errs[lineKey(i)] = "種目は30文字、ラウンドは20文字までです"
			continue
		}
		rows = append(rows, in)
	}
	if len(rows) > maxMeetSessionRows {
		errs["csv"] = "一度に入れられるのは300行までです"
	}
	return rows, errs
}

func lineKey(i int) string {
	return "line_" + strconv.Itoa(i+1)
}

func parseMeetSessionTime(v string) (time.Time, bool) {
	for _, layout := range meetSessionLayouts {
		if t, err := time.ParseInLocation(layout, v, jst.Zone); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// nowRunningByMeet は、大会ごとに「いま実施中」の種目を返す。
// 開始時刻が now 以前でいちばん遅い枠(同じ時刻に始まる種目はまとめて1枠)が、次の枠の開始まで実施中。
// 次の枠が無い最後の枠は、始まってから lastSessionRunning のあいだだけ実施中とする。
func nowRunningByMeet(rows []db.ListMeetSessionsRow, now time.Time) map[string][]string {
	byMeet := map[string][]db.ListMeetSessionsRow{}
	for _, r := range rows {
		byMeet[r.MeetKey] = append(byMeet[r.MeetKey], r)
	}
	out := map[string][]string{}
	for key, sessions := range byMeet {
		sort.SliceStable(sessions, func(i, j int) bool { return sessions[i].StartsAt.Before(sessions[j].StartsAt) })
		var slot time.Time
		for _, r := range sessions {
			if !r.StartsAt.After(now) {
				slot = r.StartsAt
			}
		}
		if slot.IsZero() {
			continue
		}
		var next time.Time
		for _, r := range sessions {
			if r.StartsAt.After(slot) {
				next = r.StartsAt
				break
			}
		}
		if next.IsZero() {
			next = slot.Add(lastSessionRunning)
		}
		if !now.Before(next) {
			continue
		}
		var labels []string
		for _, r := range sessions {
			if r.StartsAt.Equal(slot) {
				labels = append(labels, sessionLabel(r.Gender, r.Event, r.Round))
			}
		}
		out[key] = labels
	}
	return out
}

type adminMeetDTO struct {
	MeetKey        string `json:"meet_key"`
	Name           string `json:"name"`
	DateLabel      string `json:"date_label"`
	SessionCount   int    `json:"session_count"`
	OfficialURL    string `json:"official_url"`
	TimetableURL   string `json:"timetable_url"`
	YouTubeVideoID string `json:"youtube_video_id"`
}

// adminListMeetSessions は、タイムテーブルを入れられる大会(これから開催か開催中の対象大会)を返す。
func (s *Server) adminListMeetSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	targets, err := s.adminMeetTargets(r)
	if err != nil {
		MapDBError(w, err)
		return
	}
	keys := make([]string, 0, len(targets))
	for _, t := range targets {
		keys = append(keys, t.key)
	}
	counts, err := s.q.CountMeetSessions(r.Context(), keys)
	if err != nil {
		MapDBError(w, err)
		return
	}
	byKey := map[string]int{}
	for _, c := range counts {
		byKey[c.MeetKey] = int(c.SessionCount)
	}
	meets := make([]adminMeetDTO, 0, len(targets))
	resources, err := s.meetResourceMap(r.Context(), keys)
	if err != nil {
		MapDBError(w, err)
		return
	}
	for _, t := range targets {
		links := resourcesForTarget(t, resources[t.key])
		meets = append(meets, adminMeetDTO{MeetKey: t.key, Name: t.name, DateLabel: t.dateLabel, SessionCount: byKey[t.key], OfficialURL: links.OfficialURL, TimetableURL: links.TimetableURL, YouTubeVideoID: links.YouTubeVideoID})
	}
	JSON(w, http.StatusOK, map[string]any{"meets": meets})
}

func (s *Server) adminMeetTargets(r *http.Request) ([]meetTarget, error) {
	now := s.now()
	today := now.In(jst.Zone).Format("2006-01-02")
	start, end := seasonRange(currentSeasonYear(now))
	return s.meetTargets(r.Context(), start, end, func(_, ends time.Time) bool { return ends.Format("2006-01-02") >= today })
}

type putMeetSessionsRequest struct {
	MeetKey string `json:"meet_key"`
	CSV     string `json:"csv"`
}

// adminPutMeetSessions は、大会のタイムテーブルを貼り付けた内容に置き換える。空にすると消える。
func (s *Server) adminPutMeetSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var req putMeetSessionsRequest
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "リクエストを読めませんでした")
		return
	}
	rows, errs := parseMeetSessionsCSV(req.CSV)
	if len(errs) > 0 {
		FieldError(w, errs)
		return
	}
	targets, err := s.adminMeetTargets(r)
	if err != nil {
		MapDBError(w, err)
		return
	}
	known := false
	for _, t := range targets {
		if t.key == req.MeetKey {
			known = true
			break
		}
	}
	if !known {
		FieldError(w, map[string]string{"meet_key": "対象の大会ではありません"})
		return
	}
	err = s.tx(r.Context(), func(q *db.Queries) error {
		if err := q.DeleteMeetSessions(r.Context(), req.MeetKey); err != nil {
			return err
		}
		for _, in := range rows {
			if err := q.InsertMeetSession(r.Context(), db.InsertMeetSessionParams{
				MeetKey: req.MeetKey, Event: in.event, Round: in.round, Gender: in.gender, StartsAt: in.startsAt,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"meet_key": req.MeetKey, "session_count": len(rows)})
}
