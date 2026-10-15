package httpx

import (
	"strings"
	"unicode/utf8"
)

// 段階を変えるときに聞く「意識的にやったこと」。復帰までに何をしたかを集計して次の人のヒントにする。
var injuryActions = []injuryOption{
	{"hospital-rehab", "病院のリハビリ"},
	{"osteopath", "整骨院"},
	{"stretch", "ストレッチ"},
	{"bath", "入浴"},
	{"strength", "筋トレ・補強"},
	{"icing", "アイシング"},
	{"taping", "テーピング"},
	{"rest", "できるだけ動かなかった"},
	{"nothing", "何もしなかった"},
}

const injuryStageNoteMaxLength = 300

type injuryGroupStageReq struct {
	Stage   string   `json:"stage"`
	Actions []string `json:"actions"`
	// Note is the optional "気づいたこと・次の人へ一言". NotePublic lets it appear, without a nickname, on the public group page.
	Note       string `json:"note"`
	NotePublic bool   `json:"note_public"`
}

type injuryStageChange struct {
	Stage      string
	Actions    []string
	Note       *string
	NotePublic bool
}

func normalizeInjuryStageChange(req injuryGroupStageReq) (injuryStageChange, map[string]string) {
	fields := map[string]string{}
	out := injuryStageChange{Actions: []string{}}
	out.Stage = strings.TrimSpace(req.Stage)
	if !validInjuryStage(out.Stage) {
		fields["stage"] = "いまの段階を選んでください"
	}
	seen := map[string]bool{}
	for _, code := range req.Actions {
		code = strings.TrimSpace(code)
		if !validInjuryOption(injuryActions, code) {
			fields["actions"] = "意識的にやったことを選び直してください"
			break
		}
		if !seen[code] {
			seen[code] = true
			out.Actions = append(out.Actions, code)
		}
	}
	switch {
	case fields["actions"] != "":
	case len(out.Actions) == 0:
		fields["actions"] = "意識的にやったことを1つ以上選んでください(なければ「何もしなかった」)"
	case seen["nothing"] && len(out.Actions) > 1:
		fields["actions"] = "「何もしなかった」はほかと一緒に選べません"
	}
	note := strings.TrimSpace(req.Note)
	switch {
	case note == "":
		if req.NotePublic {
			fields["note_public"] = "公開するには一言を書いてください"
		}
	case utf8.RuneCountInString(note) > injuryStageNoteMaxLength:
		fields["note"] = "一言は300文字以内で書いてください"
	default:
		out.Note = &note
		out.NotePublic = req.NotePublic
	}
	return out, fields
}
