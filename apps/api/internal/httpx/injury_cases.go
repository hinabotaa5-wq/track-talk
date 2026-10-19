package httpx

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ケガ部の「データ」タブ。ケガをしたときの状況と、そのときどうすればよかったかを、選択式だけで集めて匿名で集計する。
// 自由記述は受けない(未成年の個人情報が入らないように)。回答には誰のものかを出さず、日付も返さない。

type injuryChoice struct{ Code, Label string }

var injuryCaseTimings = []injuryChoice{
	{"practice", "練習中"},
	{"warmup", "ウォームアップ中"},
	{"race", "試合・大会中"},
	{"daily", "日常生活"},
	{"other", "その他"},
}

var injuryCaseSituations = []injuryChoice{
	{"fatigue", "疲れがたまっていた"},
	{"load_up", "練習量や強度が急に増えた"},
	{"short_warmup", "ウォームアップが短かった"},
	{"pain_ignored", "違和感を我慢して続けた"},
	{"bad_condition", "睡眠不足や体調不良だった"},
	{"gear_surface", "シューズや路面が合っていなかった"},
	{"first_hard", "久しぶりに全力を出した"},
	{"weather", "寒さや暑さが厳しかった"},
	{"other", "その他"},
}

var injuryCasePreventions = []injuryChoice{
	{"longer_warmup", "ウォームアップを念入りにする"},
	{"rest_early", "違和感の段階で休む"},
	{"see_doctor_early", "早めに病院へ行く"},
	{"adjust_load", "練習量や強度を調整する"},
	{"sleep_nutrition", "睡眠や食事を整える"},
	{"gear_surface", "シューズや路面を見直す"},
	{"strength_stretch", "筋トレやストレッチを続ける"},
	{"tell_coach", "顧問やコーチに早めに伝える"},
	{"other", "その他"},
}

const (
	maxInjuryCasesPerUser = 10
	maxInjuryCaseChoices  = 4
)

func choiceLabel(list []injuryChoice, code string) string {
	for _, item := range list {
		if item.Code == code {
			return item.Label
		}
	}
	return ""
}

// normalizeChoices は、選択肢の並びを検める。空・重複・知らない値・多すぎる選択は問題として返す。
func normalizeChoices(list []injuryChoice, values []string) ([]string, bool) {
	if len(values) == 0 || len(values) > maxInjuryCaseChoices {
		return nil, false
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if choiceLabel(list, v) == "" || seen[v] {
			return nil, false
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, true
}

type injuryCaseReq struct {
	BodyPart    string   `json:"body_part"`
	Timing      string   `json:"timing"`
	Situations  []string `json:"situations"`
	Preventions []string `json:"preventions"`
}

type injuryCaseInput struct {
	BodyPart    string
	Timing      string
	Situations  []string
	Preventions []string
}

func normalizeInjuryCase(req injuryCaseReq) (injuryCaseInput, map[string]string) {
	fields := map[string]string{}
	in := injuryCaseInput{BodyPart: req.BodyPart, Timing: req.Timing}
	if !validInjuryBodyPart(req.BodyPart) {
		fields["body_part"] = "ケガを選んでください"
	}
	if choiceLabel(injuryCaseTimings, req.Timing) == "" {
		fields["timing"] = "ケガをしたときを選んでください"
	}
	var ok bool
	if in.Situations, ok = normalizeChoices(injuryCaseSituations, req.Situations); !ok {
		fields["situations"] = "そのときの状況を1〜4つ選んでください"
	}
	if in.Preventions, ok = normalizeChoices(injuryCasePreventions, req.Preventions); !ok {
		fields["preventions"] = "どうすればよかったかを1〜4つ選んでください"
	}
	return in, fields
}

type injuryChoiceStatDTO struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Count int64  `json:"count"`
	// Percent は、回答した人のうち、この選択肢を選んだ人の割合(四捨五入)。
	Percent int `json:"percent"`
}

type injuryCaseGroupDTO struct {
	// BodyPart は部位の符号。全体の集計では空。
	BodyPart      string `json:"body_part"`
	BodyPartLabel string `json:"body_part_label"`
	Cases         int64  `json:"cases"`
	// Ready が false の間は、1人の回答がそのまま見えてしまうので、選択肢ごとの数は返さない。
	Ready       bool                  `json:"ready"`
	Timings     []injuryChoiceStatDTO `json:"timings"`
	Situations  []injuryChoiceStatDTO `json:"situations"`
	Preventions []injuryChoiceStatDTO `json:"preventions"`
}

type injuryCaseStatsDTO struct {
	MinSample int                  `json:"min_sample"`
	Overall   injuryCaseGroupDTO   `json:"overall"`
	Parts     []injuryCaseGroupDTO `json:"parts"`
}

// injuryCaseTally は、集計の途中の数。キーは符号。
type injuryCaseTally struct {
	cases       int64
	timings     map[string]int64
	situations  map[string]int64
	preventions map[string]int64
}

func newInjuryCaseTally() *injuryCaseTally {
	return &injuryCaseTally{timings: map[string]int64{}, situations: map[string]int64{}, preventions: map[string]int64{}}
}

func (t *injuryCaseTally) add(o *injuryCaseTally) {
	t.cases += o.cases
	for _, pair := range []struct{ dst, src map[string]int64 }{{t.timings, o.timings}, {t.situations, o.situations}, {t.preventions, o.preventions}} {
		for k, v := range pair.src {
			pair.dst[k] += v
		}
	}
}

func choiceStats(list []injuryChoice, counts map[string]int64, cases int64) []injuryChoiceStatDTO {
	out := make([]injuryChoiceStatDTO, 0, len(counts))
	for _, item := range list {
		n := counts[item.Code]
		if n == 0 {
			continue
		}
		out = append(out, injuryChoiceStatDTO{Code: item.Code, Label: item.Label, Count: n, Percent: int((n*100 + cases/2) / cases)})
	}
	// 多い順。同数は定義順のまま。「その他」は常に最後。
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Code == "other") != (out[j].Code == "other") {
			return out[j].Code == "other"
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func buildInjuryCaseGroup(code, label string, t *injuryCaseTally) injuryCaseGroupDTO {
	g := injuryCaseGroupDTO{
		BodyPart: code, BodyPartLabel: label, Cases: t.cases, Ready: t.cases >= minInjuryStatSample,
		Timings: []injuryChoiceStatDTO{}, Situations: []injuryChoiceStatDTO{}, Preventions: []injuryChoiceStatDTO{},
	}
	if g.Ready {
		g.Timings = choiceStats(injuryCaseTimings, t.timings, t.cases)
		g.Situations = choiceStats(injuryCaseSituations, t.situations, t.cases)
		g.Preventions = choiceStats(injuryCasePreventions, t.preventions, t.cases)
	}
	return g
}

// buildInjuryCaseStats は、部位の定義順に並べ、回答のない部位は飛ばす。
func buildInjuryCaseStats(byPart map[string]*injuryCaseTally) injuryCaseStatsDTO {
	out := injuryCaseStatsDTO{MinSample: minInjuryStatSample, Parts: []injuryCaseGroupDTO{}}
	overall := newInjuryCaseTally()
	for _, part := range injuryBodyParts {
		t := byPart[part.Code]
		if t == nil || t.cases == 0 {
			continue
		}
		overall.add(t)
		out.Parts = append(out.Parts, buildInjuryCaseGroup(part.Code, part.Label, t))
	}
	out.Overall = buildInjuryCaseGroup("", "すべてのケガ", overall)
	return out
}

func (s *Server) loadInjuryCaseTallies(ctx context.Context) (map[string]*injuryCaseTally, error) {
	tallies := map[string]*injuryCaseTally{}
	get := func(part string) *injuryCaseTally {
		if tallies[part] == nil {
			tallies[part] = newInjuryCaseTally()
		}
		return tallies[part]
	}
	rows, err := s.pool.Query(ctx, `SELECT body_part, count(*)::bigint FROM injury_cases WHERE hidden_at IS NULL GROUP BY body_part`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var part string
		var n int64
		if err := rows.Scan(&part, &n); err != nil {
			rows.Close()
			return nil, err
		}
		get(part).cases = n
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	for _, q := range []struct {
		sql string
		dst func(*injuryCaseTally) map[string]int64
	}{
		{`SELECT body_part, timing, count(*)::bigint FROM injury_cases WHERE hidden_at IS NULL GROUP BY body_part, timing`, func(t *injuryCaseTally) map[string]int64 { return t.timings }},
		{`SELECT body_part, v, count(*)::bigint FROM injury_cases, unnest(situations) AS v WHERE hidden_at IS NULL GROUP BY body_part, v`, func(t *injuryCaseTally) map[string]int64 { return t.situations }},
		{`SELECT body_part, v, count(*)::bigint FROM injury_cases, unnest(preventions) AS v WHERE hidden_at IS NULL GROUP BY body_part, v`, func(t *injuryCaseTally) map[string]int64 { return t.preventions }},
	} {
		rows, err := s.pool.Query(ctx, q.sql)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var part, code string
			var n int64
			if err := rows.Scan(&part, &code, &n); err != nil {
				rows.Close()
				return nil, err
			}
			q.dst(get(part))[code] = n
		}
		rows.Close()
		if rows.Err() != nil {
			return nil, rows.Err()
		}
	}
	return tallies, nil
}

// injuryCaseStats は、ログインなしで見られる匿名の集計。
func (s *Server) injuryCaseStats(w http.ResponseWriter, r *http.Request) {
	tallies, err := s.loadInjuryCaseTallies(r.Context())
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusOK, buildInjuryCaseStats(tallies))
}

type injuryMyCaseDTO struct {
	ID            uuid.UUID `json:"id"`
	BodyPart      string    `json:"body_part"`
	BodyPartLabel string    `json:"body_part_label"`
	TimingLabel   string    `json:"timing_label"`
	Situations    []string  `json:"situations"`
	Preventions   []string  `json:"preventions"`
	CreatedAt     time.Time `json:"created_at"`
}

func labelsOf(list []injuryChoice, codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		out = append(out, choiceLabel(list, c))
	}
	return out
}

// createInjuryCase は、1人10件まで。回答はあとから本人だけが消せる。
func (s *Server) createInjuryCase(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	var req injuryCaseReq
	if err := DecodeJSON(r, &req); err != nil {
		Error(w, http.StatusBadRequest, "validation_error", "JSONが不正です")
		return
	}
	in, fields := normalizeInjuryCase(req)
	if len(fields) > 0 {
		FieldError(w, fields)
		return
	}
	var id uuid.UUID
	var createdAt time.Time
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO injury_cases (user_id, body_part, timing, situations, preventions)
		SELECT $1, $2, $3, $4, $5
		WHERE (SELECT count(*) FROM injury_cases WHERE user_id = $1) < $6
		RETURNING id, created_at`,
		p.ID, in.BodyPart, in.Timing, in.Situations, in.Preventions, maxInjuryCasesPerUser).Scan(&id, &createdAt)
	if err == pgx.ErrNoRows {
		Error(w, http.StatusConflict, "limit_reached", "回答できるのは10件までです。古い回答を消してから送ってください")
		return
	}
	if err != nil {
		MapDBError(w, err)
		return
	}
	JSON(w, http.StatusCreated, injuryMyCaseDTO{
		ID: id, BodyPart: in.BodyPart, BodyPartLabel: injuryBodyPartLabel(in.BodyPart),
		TimingLabel: choiceLabel(injuryCaseTimings, in.Timing),
		Situations:  labelsOf(injuryCaseSituations, in.Situations), Preventions: labelsOf(injuryCasePreventions, in.Preventions),
		CreatedAt: createdAt,
	})
}

// listMyInjuryCases は、自分の回答だけを返す。
func (s *Server) listMyInjuryCases(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, body_part, timing, situations, preventions, created_at
		FROM injury_cases WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, p.ID, maxInjuryCasesPerUser)
	if err != nil {
		MapDBError(w, err)
		return
	}
	defer rows.Close()
	items := []injuryMyCaseDTO{}
	for rows.Next() {
		var item injuryMyCaseDTO
		var timing string
		var situations, preventions []string
		if err := rows.Scan(&item.ID, &item.BodyPart, &timing, &situations, &preventions, &item.CreatedAt); err != nil {
			MapDBError(w, err)
			return
		}
		item.BodyPartLabel = injuryBodyPartLabel(item.BodyPart)
		item.TimingLabel = choiceLabel(injuryCaseTimings, timing)
		item.Situations = labelsOf(injuryCaseSituations, situations)
		item.Preventions = labelsOf(injuryCasePreventions, preventions)
		items = append(items, item)
	}
	if rows.Err() != nil {
		MapDBError(w, rows.Err())
		return
	}
	JSON(w, http.StatusOK, map[string]any{"cases": items})
}

func (s *Server) deleteInjuryCase(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requireProfile(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusNotFound, "not_found", "回答が見つかりません")
		return
	}
	tag, err := s.pool.Exec(r.Context(), `DELETE FROM injury_cases WHERE id = $1 AND user_id = $2`, id, p.ID)
	if err != nil {
		MapDBError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		Error(w, http.StatusNotFound, "not_found", "回答が見つかりません")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// injuryCaseOptions は、フォームの選択肢。画面と同じ符号・文言を API から返し、ずれを防ぐ。
func (s *Server) injuryCaseOptions(w http.ResponseWriter, r *http.Request) {
	conv := func(list []injuryChoice) []map[string]string {
		out := make([]map[string]string, 0, len(list))
		for _, c := range list {
			out = append(out, map[string]string{"value": c.Code, "label": c.Label})
		}
		return out
	}
	JSON(w, http.StatusOK, map[string]any{
		"timings": conv(injuryCaseTimings), "situations": conv(injuryCaseSituations), "preventions": conv(injuryCasePreventions),
		"max_choices": maxInjuryCaseChoices,
	})
}
