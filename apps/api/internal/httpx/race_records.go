package httpx

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hinata/track-talk/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
)

// 競技会の記録は本人だけが読み書きする。感覚メモと同じく、フィードにも公開ページにも出さない。
const (
	maxRaceRecordMemo = 300
	maxRaceRecords    = 1000
	// 追い風参考の境目(0.1m/s単位)。+2.0m/s を超えると公認されない。
	maxLegalWind = 20
)

// raceMarkKind は、種目の記録をどう保存して比べるか。
type raceMarkKind string

const (
	// raceMarkTime は 1/100 秒単位。小さいほどよい。
	raceMarkTime raceMarkKind = "time"
	// raceMarkDistance はセンチメートル単位。大きいほどよい。
	raceMarkDistance raceMarkKind = "distance"
	// raceMarkPoints は混成競技の得点。大きいほどよい。
	raceMarkPoints raceMarkKind = "points"
)

// raceEvents は、記録を付けられる練習の種目（"other" は単位が分からないので外す）。
// maxMark は、打ち間違い（桁が1つ多いなど）がグラフに入らないようにする。
var raceEvents = map[string]struct {
	kind    raceMarkKind
	wind    bool
	maxMark int
}{
	"100m":        {raceMarkTime, true, 6000},
	"200m":        {raceMarkTime, true, 12000},
	"400m":        {raceMarkTime, false, 24000},
	"800m":        {raceMarkTime, false, 60000},
	"1500m":       {raceMarkTime, false, 120000},
	"5000m":       {raceMarkTime, false, 360000},
	"10000m":      {raceMarkTime, false, 720000},
	"100mh":       {raceMarkTime, true, 6000},
	"110mh":       {raceMarkTime, true, 6000},
	"400mh":       {raceMarkTime, false, 24000},
	"3000msc":     {raceMarkTime, false, 240000},
	"4x100":       {raceMarkTime, false, 24000},
	"4x400":       {raceMarkTime, false, 90000},
	"long-jump":   {raceMarkDistance, true, 1200},
	"triple-jump": {raceMarkDistance, true, 2000},
	"high-jump":   {raceMarkDistance, false, 300},
	"pole-vault":  {raceMarkDistance, false, 700},
	"shot-put":    {raceMarkDistance, false, 3000},
	"discus":      {raceMarkDistance, false, 8000},
	"hammer":      {raceMarkDistance, false, 9000},
	"javelin":     {raceMarkDistance, false, 11000},
	"heptathlon":  {raceMarkPoints, false, 9000},
	"decathlon":   {raceMarkPoints, false, 10000},
}

type raceRecordReq struct {
	RacedOn string `json:"raced_on"`
	Event   string `json:"event"`
	Mark    int    `json:"mark"`
	Wind    *int   `json:"wind"`
	Memo    string `json:"memo"`
}

type raceRecordDTO struct {
	ID         uuid.UUID `json:"id"`
	RacedOn    string    `json:"raced_on"`
	Event      string    `json:"event"`
	EventLabel string    `json:"event_label"`
	Mark       int32     `json:"mark"`
	Wind       *int16    `json:"wind"`
	// WindAided は追い風 +2.0m/s を超えた記録の印。そういう記録は自己ベストに数えない。
	WindAided bool `json:"wind_aided"`
	// IsPB は、同じ種目のそれまでの公認記録をすべて上回ったとき true。
	IsPB      bool      `json:"is_pb"`
	Memo      string    `json:"memo"`
	CreatedAt time.Time `json:"created_at"`
}

type raceEventDTO struct {
	Event      string       `json:"event"`
	EventLabel string       `json:"event_label"`
	Kind       raceMarkKind `json:"kind"`
	Wind       bool         `json:"wind"`
	Count      int          `json:"count"`
}

type raceRecordsDTO struct {
	Events  []raceEventDTO  `json:"events"`
	Records []raceRecordDTO `json:"records"`
}

func normalizeRaceRecord(req raceRecordReq, today time.Time) (db.CreateRaceRecordParams, map[string]string) {
	fields := map[string]string{}
	var out db.CreateRaceRecordParams

	day, err := time.Parse(time.DateOnly, strings.TrimSpace(req.RacedOn))
	switch {
	case err != nil:
		fields["raced_on"] = "日付を選んでください"
	case day.After(today):
		fields["raced_on"] = "これから先の日付は選べません"
	case day.Year() < 1950:
		fields["raced_on"] = "日付を選び直してください"
	default:
		out.RacedOn = day
	}

	event := strings.TrimSpace(req.Event)
	spec, ok := raceEvents[event]
	if !ok {
		fields["event"] = "種目を選び直してください"
	} else {
		out.Event = event
		if req.Mark <= 0 || req.Mark > spec.maxMark {
			fields["mark"] = "記録を入力し直してください"
		} else {
			out.Mark = int32(req.Mark)
		}
		if req.Wind != nil {
			switch {
			case !spec.wind:
				fields["wind"] = "この種目に風は入力できません"
			case *req.Wind < -99 || *req.Wind > 99:
				fields["wind"] = "風は-9.9〜+9.9m/sで入力してください"
			default:
				w := int16(*req.Wind)
				out.Wind = &w
			}
		}
	}

	memo := strings.TrimSpace(req.Memo)
	if len([]rune(memo)) > maxRaceRecordMemo {
		fields["memo"] = "感想は300文字以内で書いてください"
	}
	out.Memo = memo
	return out, fields
}

// better は、種目の kind で a が b を上回るかを返す。
func (k raceMarkKind) better(a, b int32) bool {
	if k == raceMarkTime {
		return a < b
	}
	return a > b
}

// buildRaceRecords は、追い風参考の記録と自己ベストに印を付ける。rows は種目、日付の順に並んでいること。
func buildRaceRecords(rows []db.ListRaceRecordsRow) raceRecordsDTO {
	out := raceRecordsDTO{Events: []raceEventDTO{}, Records: make([]raceRecordDTO, 0, len(rows))}
	best := map[string]int32{}
	for _, row := range rows {
		spec := raceEvents[row.Event]
		item := raceRecordDTO{
			ID:         row.ID,
			RacedOn:    row.RacedOn.Format(time.DateOnly),
			Event:      row.Event,
			EventLabel: practiceLabel(row.Event, "その他"),
			Mark:       row.Mark,
			Wind:       row.Wind,
			WindAided:  row.Wind != nil && *row.Wind > maxLegalWind,
			Memo:       row.Memo,
			CreatedAt:  row.CreatedAt,
		}
		if !item.WindAided {
			if prev, ok := best[row.Event]; !ok || spec.kind.better(row.Mark, prev) {
				best[row.Event] = row.Mark
				item.IsPB = true
			}
		}
		if n := len(out.Events); n == 0 || out.Events[n-1].Event != row.Event {
			out.Events = append(out.Events, raceEventDTO{Event: row.Event, EventLabel: item.EventLabel, Kind: spec.kind, Wind: spec.wind})
		}
		out.Events[len(out.Events)-1].Count++
		out.Records = append(out.Records, item)
	}
	return out
}

func (s *Server) writeRaceRecords(w http.ResponseWriter, r *http.Request, authorID uuid.UUID, status int) {
	rows, err := s.q.ListRaceRecords(r.Context(), authorID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, status, buildRaceRecords(rows))
}

func (s *Server) listRaceRecords(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	s.writeRaceRecords(w, r, me.ID, http.StatusOK)
}

func (s *Server) createRaceRecord(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req raceRecordReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	params, fields := normalizeRaceRecord(req, tokyoToday())
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	count, err := s.q.CountRaceRecords(r.Context(), me.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if count >= maxRaceRecords {
		Error(w, http.StatusUnprocessableEntity, "validation_error", "記録は1000件までです")
		return
	}
	params.ID, params.AuthorID = uuid.New(), me.ID
	if _, err := s.q.CreateRaceRecord(r.Context(), params); err != nil {
		MapDBError(w, err)
		return
	}
	s.writeRaceRecords(w, r, me.ID, http.StatusCreated)
}

func (s *Server) updateRaceRecord(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var req raceRecordReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	params, fields := normalizeRaceRecord(req, tokyoToday())
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	_, err := s.q.UpdateRaceRecord(r.Context(), db.UpdateRaceRecordParams{
		ID:       id,
		AuthorID: me.ID,
		RacedOn:  params.RacedOn,
		Event:    params.Event,
		Mark:     params.Mark,
		Wind:     params.Wind,
		Memo:     params.Memo,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			Error(w, http.StatusNotFound, "not_found", "記録が見つかりません")
			return
		}
		MapDBError(w, err)
		return
	}
	s.writeRaceRecords(w, r, me.ID, http.StatusOK)
}

func (s *Server) deleteRaceRecord(w http.ResponseWriter, r *http.Request) {
	me, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	n, err := s.q.DeleteRaceRecord(r.Context(), db.DeleteRaceRecordParams{ID: id, AuthorID: me.ID})
	if err != nil {
		MapDBError(w, err)
		return
	}
	if n == 0 {
		Error(w, http.StatusNotFound, "not_found", "記録が見つかりません")
		return
	}
	s.writeRaceRecords(w, r, me.ID, http.StatusOK)
}
