package httpx

// appEvents は、アプリ内で選べる種目。コーチの担当種目と同じ一覧と並びで、コーチ・練習相手募集・感覚メモ・
// 競技会の記録・レビュー・利用者の種目(登録のときに選ぶ)・ケガ部がすべてこれを使う。
var appEvents = []struct {
	Code  string
	Label string
}{
	{"100m", "100m"},
	{"200m", "200m"},
	{"400m", "400m"},
	{"800m", "800m"},
	{"1500m", "1500m"},
	{"5000m", "5000m"},
	{"10000m", "10000m"},
	{"110mh", "110mハードル"},
	{"100mh", "100mハードル"},
	{"400mh", "400mハードル"},
	{"3000msc", "3000m障害物"},
	{"high-jump", "走高跳"},
	{"pole-vault", "棒高跳"},
	{"long-jump", "走幅跳"},
	{"triple-jump", "三段跳"},
	{"shot-put", "砲丸投"},
	{"discus", "円盤投"},
	{"hammer", "ハンマー投"},
	{"javelin", "やり投"},
	{"race-walk", "競歩"},
	{"relay", "リレー"},
}

// legacyEventLabels は、いまは選べないが、以前に保存した値の表示に使う名前。
var legacyEventLabels = map[string]string{
	"heptathlon": "七種競技",
	"decathlon":  "十種競技",
	"4x100":      "4×100mR",
	"4x400":      "4×400mR",
	"sprint":     "短距離",
	"hurdles":    "ハードル",
	"middle":     "中距離",
	"long":       "長距離",
	"jumps":      "跳躍",
	"throws":     "投てき",
	"combined":   "混成",
}

func validEvent(code string) bool {
	for _, event := range appEvents {
		if event.Code == code {
			return true
		}
	}
	return false
}

// eventLabel は種目の表示名。選べなくなった種目も、保存してある値なら表示できる。知らない値はそのまま返す。
func eventLabel(code string) string {
	for _, event := range appEvents {
		if event.Code == code {
			return event.Label
		}
	}
	if label, ok := legacyEventLabels[code]; ok {
		return label
	}
	return code
}

// eventOptions は、種目を選択肢の形にしたもの(ケガ部の集計の表示名に使う)。以前の区分も含む。
func eventOptions() []injuryOption {
	out := make([]injuryOption, 0, len(appEvents)+len(legacyEventLabels))
	for _, event := range appEvents {
		out = append(out, injuryOption{event.Code, event.Label})
	}
	for _, code := range []string{"sprint", "hurdles", "middle", "long", "jumps", "throws", "combined", "heptathlon", "decathlon", "4x100", "4x400"} {
		out = append(out, injuryOption{code, legacyEventLabels[code]})
	}
	return out
}
