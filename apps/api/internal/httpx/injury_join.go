package httpx

import (
	"strings"
	"time"
	"unicode/utf8"
)

type injuryOption struct{ Code, Label string }

// 入るときに聞く選択肢。予防や復帰の目安に集計するので、自由記述ではなく選んでもらう。
var (
	injurySeverities = []injuryOption{
		{"mild", "軽い(練習しながら様子を見ている)"},
		{"moderate", "中くらい(練習を数週間休む)"},
		{"severe", "重い(1か月以上休む、または手術)"},
	}
	injuryPlaces = []injuryOption{
		{"tartan", "タータン"},
		{"grass", "芝生"},
		{"dirt", "土"},
		{"asphalt", "アスファルト"},
		{"slope", "坂"},
		{"stairs", "階段"},
		{"weight-room", "ウエイト場"},
		{"gym", "体育館"},
	}
	injuryActivities = []injuryOption{
		{"jog", "ジョグ"},
		{"start", "スタート・加速"},
		{"sprint", "中間から後半を全力疾走"},
		{"jump-hurdle", "ジャンプ・ハードル"},
		{"throw", "投てき"},
		{"weights", "ウエイト"},
		{"outside", "練習以外"},
	}
	injuryPreChanges = []injuryOption{
		{"more-volume", "練習量を増やした"},
		{"new-shoes", "シューズを替えた"},
		{"comeback", "休み明け・久しぶりの再開"},
		{"fatigue", "疲れがたまっていた"},
		{"played-through-pain", "痛みをがまんしていた"},
		{"none", "とくになし"},
	}
	injuryEvents = []injuryOption{
		{"sprint", "短距離"},
		{"hurdles", "ハードル"},
		{"middle", "中距離"},
		{"long", "長距離"},
		{"jumps", "跳躍"},
		{"throws", "投てき"},
		{"combined", "混成"},
		{"race-walk", "競歩"},
	}
)

const injuryDiagnosisMaxLength = 40

func injuryOptionLabel(options []injuryOption, code string) string {
	for _, item := range options {
		if item.Code == code {
			return item.Label
		}
	}
	return ""
}

func validInjuryOption(options []injuryOption, code string) bool {
	return injuryOptionLabel(options, code) != ""
}

type injuryJoinReq struct {
	// Diagnosis を聞くのは "other" のグループだけ。そこではグループ自体がケガの名前を表していないから。
	Diagnosis    string   `json:"diagnosis"`
	Severity     string   `json:"severity"`
	SawDoctor    *bool    `json:"saw_doctor"`
	InjuredMonth string   `json:"injured_month"`
	Place        string   `json:"place"`
	Activity     string   `json:"activity"`
	PreChanges   []string `json:"pre_changes"`
	Event        string   `json:"event"`
	Stage        string   `json:"stage"`
}

type injuryJoin struct {
	Diagnosis  *string
	Severity   string
	SawDoctor  bool
	InjuredOn  time.Time
	Place      string
	Activity   string
	PreChanges []string
	Event      string
	Stage      string
}

// normalizeInjuryJoin は答えを確かめる。月はその月の1日として保存する。テストで固定できるよう、now は引数で受け取る。
func normalizeInjuryJoin(part string, req injuryJoinReq, now time.Time) (injuryJoin, map[string]string) {
	fields := map[string]string{}
	out := injuryJoin{}
	if part == "other" {
		diagnosis := strings.TrimSpace(req.Diagnosis)
		if n := utf8.RuneCountInString(diagnosis); n < 1 || n > injuryDiagnosisMaxLength {
			fields["diagnosis"] = "ケガの名前を40文字以内で入力してください(わからなければ「わからない」)"
		} else {
			out.Diagnosis = &diagnosis
		}
	}
	out.Severity = strings.TrimSpace(req.Severity)
	if !validInjuryOption(injurySeverities, out.Severity) {
		fields["severity"] = "ケガの重さを選んでください"
	}
	if req.SawDoctor == nil {
		fields["saw_doctor"] = "病院でみてもらったかを選んでください"
	} else {
		out.SawDoctor = *req.SawDoctor
	}
	month, err := time.Parse("2006-01", strings.TrimSpace(req.InjuredMonth))
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	switch {
	case err != nil:
		fields["injured_month"] = "いつごろケガをしたかを選んでください"
	case month.After(thisMonth):
		fields["injured_month"] = "これから先の月は選べません"
	default:
		out.InjuredOn = month
	}
	out.Place = strings.TrimSpace(req.Place)
	if !validInjuryOption(injuryPlaces, out.Place) {
		fields["place"] = "ケガをした場所を選んでください"
	}
	out.Activity = strings.TrimSpace(req.Activity)
	if !validInjuryOption(injuryActivities, out.Activity) {
		fields["activity"] = "そのとき何をしていたかを選んでください"
	}
	seen := map[string]bool{}
	for _, code := range req.PreChanges {
		code = strings.TrimSpace(code)
		if !validInjuryOption(injuryPreChanges, code) {
			fields["pre_changes"] = "ケガの前に変わったことを選び直してください"
			break
		}
		if !seen[code] {
			seen[code] = true
			out.PreChanges = append(out.PreChanges, code)
		}
	}
	switch {
	case fields["pre_changes"] != "":
	case len(out.PreChanges) == 0:
		fields["pre_changes"] = "ケガの前に変わったことを選んでください(なければ「とくになし」)"
	case seen["none"] && len(out.PreChanges) > 1:
		fields["pre_changes"] = "「とくになし」はほかと一緒に選べません"
	}
	out.Event = strings.TrimSpace(req.Event)
	if !validInjuryOption(injuryEvents, out.Event) {
		fields["event"] = "種目を選んでください"
	}
	out.Stage = strings.TrimSpace(req.Stage)
	if !validInjuryStage(out.Stage) {
		fields["stage"] = "いまの段階を選んでください"
	}
	return out, fields
}
