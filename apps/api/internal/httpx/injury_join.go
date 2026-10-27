package httpx

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hinata/track-talk/apps/api/internal/jst"
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
		{"asphalt", "アスファルト"},
		{"slope", "坂"},
		{"stairs", "階段"},
		{"weight-room", "ジム"},
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
		{"played-through-pain", "痛みを我慢していた"},
		{"none", "とくになし"},
	}
	// 「いま思うと、どうすればよかったか」。まだケガをしていない人の予防に使う。
	injuryHindsights = []injuryOption{
		{"rest-early", "違和感の段階で休む"},
		{"warmup", "ウォームアップを念入りにする"},
		{"adjust-volume", "練習量や強度を調整する"},
		{"see-doctor-early", "早めに病院へ行く"},
		{"sleep-food", "睡眠や食事を整える"},
		{"gear", "シューズや路面を見直す"},
		{"tell-coach", "顧問やコーチに早めに伝える"},
		{"none", "わからない・とくにない"},
	}
	// 種目は登録のときに選んだものを使う。以前の区分で答えた人の値も表示できるようにしておく。
	injuryEvents = eventOptions()
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
	Diagnosis  string   `json:"diagnosis"`
	Severity   string   `json:"severity"`
	SawDoctor  *bool    `json:"saw_doctor"`
	InjuredOn  string   `json:"injured_on"`
	Place      string   `json:"place"`
	Activity   string   `json:"activity"`
	PreChanges []string `json:"pre_changes"`
	// Hindsight は、前から入っている人の更新やむかしの画面からも来るので、空でもよい。
	Hindsight []string `json:"hindsight"`
	Stage     string   `json:"stage"`
	// HealthConsent は、ケガの情報を健康に関する情報として扱うことへの同意(要配慮個人情報なので、取る前に同意がいる)。
	HealthConsent bool `json:"health_consent"`
}

type injuryJoin struct {
	Diagnosis  *string
	Severity   string
	SawDoctor  bool
	InjuredOn  time.Time
	Place      string
	Activity   string
	PreChanges []string
	Hindsight  []string
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
	day, err := time.Parse("2006-01-02", strings.TrimSpace(req.InjuredOn))
	// 「今日」は日本時間で決める（UTC のままだと、毎日の朝9時前は前日になる）。
	now = now.In(jst.Zone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch {
	case err != nil:
		fields["injured_on"] = "いつケガをしたかを選んでください"
	case day.After(today):
		fields["injured_on"] = "これから先の日は選べません"
	case day.Before(today.AddDate(-10, 0, 0)):
		fields["injured_on"] = "10年より前の日は選べません"
	default:
		out.InjuredOn = day
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
	hindsight := map[string]bool{}
	for _, code := range req.Hindsight {
		code = strings.TrimSpace(code)
		if !validInjuryOption(injuryHindsights, code) {
			fields["hindsight"] = "どうすればよかったかを選び直してください"
			break
		}
		if !hindsight[code] {
			hindsight[code] = true
			out.Hindsight = append(out.Hindsight, code)
		}
	}
	if fields["hindsight"] == "" && hindsight["none"] && len(out.Hindsight) > 1 {
		fields["hindsight"] = "「わからない・とくにない」はほかと一緒に選べません"
	}
	out.Stage = strings.TrimSpace(req.Stage)
	if !validInjuryStage(out.Stage) {
		fields["stage"] = "いまの段階を選んでください"
	}
	if !req.HealthConsent {
		fields["health_consent"] = "ケガの情報の取り扱いに同意してください"
	}
	return out, fields
}
