package competitions

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// localizeMeetName translates a known series, meeting, or road race into Japanese.
// Titles that are not in the table stay as written.
func localizeMeetName(name string) string {
	original := strings.TrimSpace(name)
	normalized := normalizeTitle(original)
	if normalized == "" {
		return original
	}
	if ja, ok := containedMeeting(foldTitle(normalized)); ok {
		return ja
	}
	for _, series := range seriesByLength {
		before, after, ok := cutSeries(normalized, series.en)
		if !ok {
			continue
		}
		rest := joinCities(stripTrailingYear(strings.TrimSpace(before + " " + after)))
		if rest == "" {
			return series.ja
		}
		return series.ja + "・" + rest
	}
	return original
}

func localizeLocation(location string) string {
	original := strings.TrimSpace(location)
	if original == "" {
		return ""
	}
	body, country, ok := splitCountry(original)
	if !ok {
		body = original
	}
	cities := joinCities(body)
	if cities == "" {
		if !ok {
			return original
		}
		body = strings.TrimSpace(trailingStatePattern.ReplaceAllString(body, ""))
		body = strings.Trim(body, " ,-/")
		if body == "" {
			return country
		}
		return body + "（" + country + "）"
	}
	if country == "" || cities == country {
		return cities
	}
	return cities + "（" + country + "）"
}

type titled struct {
	en string
	ja string
}

type cityAlias struct {
	folded string
	ja     string
	n      int
}

var (
	seriesByLength   []titled
	meetingsByLength []cityAlias
	citiesByLength   []cityAlias
)

func init() {
	seriesByLength = append([]titled(nil), seriesNames...)
	sort.Slice(seriesByLength, func(i, j int) bool {
		return len([]rune(seriesByLength[i].en)) > len([]rune(seriesByLength[j].en))
	})
	meetingsByLength = foldTitles(meetingNames)
	citiesByLength = foldTitles(cityNames)
}

func foldTitles(items []titled) []cityAlias {
	seen := map[string]bool{}
	out := make([]cityAlias, 0, len(items))
	for _, item := range items {
		folded := foldTitle(item.en)
		if folded == "" || seen[folded] {
			continue
		}
		seen[folded] = true
		out = append(out, cityAlias{folded: folded, ja: item.ja, n: len([]rune(folded))})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].n > out[j].n
	})
	return out
}

func containedMeeting(foldedName string) (string, bool) {
	runes := []rune(foldedName)
	for _, meeting := range meetingsByLength {
		phrase := []rune(meeting.folded)
		if len(phrase) == 0 || len(phrase) > len(runes) {
			continue
		}
		for i := 0; i+len(phrase) <= len(runes); i++ {
			if string(runes[i:i+len(phrase)]) != meeting.folded {
				continue
			}
			if boundaryBefore(runes, i) && boundaryAfter(runes, i+len(phrase)) {
				return meeting.ja, true
			}
		}
	}
	return "", false
}

func cutSeries(name, en string) (string, string, bool) {
	nameRunes := []rune(name)
	foldedName := []rune(foldTitle(name))
	foldedEn := []rune(foldTitle(en))
	if len(nameRunes) != len(foldedName) || len(foldedEn) == 0 || len(foldedName) < len(foldedEn) {
		return "", "", false
	}
	for i := 0; i+len(foldedEn) <= len(foldedName); i++ {
		if string(foldedName[i:i+len(foldedEn)]) != string(foldedEn) {
			continue
		}
		if !boundaryBefore(foldedName, i) || !boundaryAfter(foldedName, i+len(foldedEn)) {
			continue
		}
		before := strings.Trim(string(nameRunes[:i]), " ,-/–—・")
		after := strings.Trim(string(nameRunes[i+len(foldedEn):]), " ,-/–—・")
		return before, after, true
	}
	return "", "", false
}

func joinCities(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	folded := []rune(foldCity(text))
	if len(runes) != len(folded) {
		return ""
	}
	var spans []citySpan
	for i := 0; i < len(folded); {
		ja, n, ok := matchCity(folded, i)
		if !ok {
			i++
			continue
		}
		spans = append(spans, citySpan{start: i, end: i + n, ja: ja})
		i += n
	}
	return joinCitySpans(runes, spans)
}

type citySpan struct {
	start int
	end   int
	ja    string
}

func joinCitySpans(runes []rune, spans []citySpan) string {
	var b strings.Builder
	prevJa := ""
	prevEnd := 0
	written := false
	for _, span := range spans {
		sep := "・"
		if written {
			gap := strings.TrimSpace(string(runes[prevEnd:span.start]))
			if gap == "/" {
				sep = "/"
			}
			if span.ja == prevJa && sep != "/" {
				prevEnd = span.end
				continue
			}
			b.WriteString(sep)
		}
		b.WriteString(span.ja)
		prevJa = span.ja
		prevEnd = span.end
		written = true
	}
	return b.String()
}

func matchCity(folded []rune, i int) (string, int, bool) {
	if !boundaryBefore(folded, i) {
		return "", 0, false
	}
	for _, city := range citiesByLength {
		end := i + city.n
		if end > len(folded) || string(folded[i:end]) != city.folded || !boundaryAfter(folded, end) {
			continue
		}
		return city.ja, city.n, true
	}
	return "", 0, false
}

func boundaryBefore(runes []rune, i int) bool {
	return i <= 0 || !unicode.IsLetter(runes[i-1])
}

func boundaryAfter(runes []rune, i int) bool {
	return i >= len(runes) || !unicode.IsLetter(runes[i])
}

func foldTitle(value string) string {
	value = strings.NewReplacer(
		"'", " ", "’", " ", "‘", " ", "`", " ", "´", " ", "ʼ", " ",
	).Replace(value)
	return foldCity(value)
}

func normalizeTitle(name string) string {
	name = strings.TrimSpace(name)
	name = stripLeadingYear(name)
	name = stripArticle(name)
	name = stripTrailingYear(name)
	name = strings.TrimSpace(presentedByPattern.ReplaceAllString(name, ""))
	return strings.TrimSpace(name)
}

func stripLeadingYear(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) < 6 || !isFourDigitYear(string(runes[:4])) || !isSep(runes[4]) {
		return strings.TrimSpace(name)
	}
	return strings.Trim(string(runes[4:]), " ,-/–—")
}

func stripArticle(name string) string {
	runes := []rune(strings.TrimSpace(name))
	folded := []rune(foldTitle(string(runes)))
	if len(runes) != len(folded) || len(folded) < 5 || string(folded[:4]) != "the " {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(string(runes[4:]))
}

func stripTrailingYear(name string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(name), " ")
	i := strings.LastIndexAny(trimmed, " -/–—,")
	if i < 0 || i >= len(trimmed)-1 || !isYearToken(trimmed[i+1:]) {
		return strings.TrimSpace(name)
	}
	return strings.Trim(trimmed[:i], " ,-/–—")
}

func isFourDigitYear(token string) bool {
	return len(token) == 4 && token[0] == '2' && token[1] == '0' && isDigits(token)
}

func isYearToken(token string) bool {
	if isFourDigitYear(token) {
		return true
	}
	return len(token) == 2 && token[0] >= '2' && token[0] <= '3' && isDigits(token)
}

func isDigits(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isSep(r rune) bool {
	switch r {
	case ' ', ',', '-', '/', '·', '・', '–', '—', ':':
		return true
	default:
		return false
	}
}

func splitCountry(location string) (string, string, bool) {
	loc := trailingCountryPattern.FindStringSubmatchIndex(location)
	if loc == nil {
		return location, "", false
	}
	country, ok := countries[location[loc[2]:loc[3]]]
	if !ok {
		return location, "", false
	}
	return strings.TrimSpace(location[:loc[0]]), country, true
}

var (
	presentedByPattern     = regexp.MustCompile(`(?i)\s+presented by\s+.+$`)
	trailingCountryPattern = regexp.MustCompile(`\s*\(([A-Z]{3})\)\s*$`)
	trailingStatePattern   = regexp.MustCompile(`,?\s*[A-Z]{2}$`)
)

var seriesNames = []titled{
	{"World Athletics Race Walking Team Championships", "世界競歩チーム選手権"},
	{"World Athletics Road Running Championships", "世界ロードランニング"},
	{"World Athletics Cross Country Championships", "世界クロスカントリー"},
	{"World Athletics Half Marathon Championships", "世界ハーフマラソン"},
	{"World Athletics Indoor Championships", "世界室内陸上"},
	{"World Athletics Ultimate Championship", "世界アルティメット選手権"},
	{"World Athletics Combined Events Tour", "混成競技ツアー"},
	{"World Athletics Continental Tour Gold", "コンチネンタルツアー・ゴールド"},
	{"World Athletics Continental Tour Silver", "コンチネンタルツアー・シルバー"},
	{"World Athletics Continental Tour Bronze", "コンチネンタルツアー・ブロンズ"},
	{"World Athletics Continental Tour", "コンチネンタルツアー"},
	{"World Athletics U20 Championships", "U20世界陸上"},
	{"World Athletics U18 Championships", "U18世界陸上"},
	{"World Athletics Championships", "世界陸上"},
	{"World Athletics Relays", "世界リレー"},
	{"Wanda Diamond League Final", "ダイヤモンドリーグ・ファイナル"},
	{"Diamond League Final", "ダイヤモンドリーグ・ファイナル"},
	{"Wanda Diamond League", "ダイヤモンドリーグ"},
	{"Diamond League", "ダイヤモンドリーグ"},
	{"Summer Olympic Games", "オリンピック"},
	{"Winter Olympic Games", "冬季オリンピック"},
	{"Olympic Games", "オリンピック"},
}

var meetingNames = []titled{
	{"Silesia Kamila Skolimowska Memorial", "カミラスコリモフスカ記念"},
	{"Kamila Skolimowska Memorial", "カミラスコリモフスカ記念"},
	{"Meeting International Mohammed VI", "モハメド6世記念"},
	{"Golden Gala Pietro Mennea", "ゴールデンガラ"},
	{"London Athletics Meet", "ダイヤモンドリーグ・ロンドン"},
	{"Memorial van Damme", "メモリアル・ヴァンダム"},
	{"Meeting de Paris", "ミーティング・ド・パリ"},
	{"Prefontaine Classic", "プレフォンテーンクラシック"},
	{"Weltklasse Zürich", "ヴェルトクラッセ・チューリッヒ"},
	{"Golden Gala", "ゴールデンガラ"},
	{"Bislett Games", "ビスレットゲームズ"},
	{"BAUHAUS-galan", "バウハウスガラン"},
	{"Athletissima", "アスレティッシマ"},
	{"Doha Meeting", "ダイヤモンドリーグ・ドーハ"},
	{"Mohammed VI", "モハメド6世記念"},
	{"Herculis", "ヘラクレス"},
	{"New York City Marathon", "ニューヨークシティマラソン"},
	{"Bank of America Chicago Marathon", "シカゴマラソン"},
	{"Chicago Marathon", "シカゴマラソン"},
	{"Amsterdam Marathon", "アムステルダムマラソン"},
	{"Sydney Marathon", "シドニーマラソン"},
	{"London Marathon", "ロンドンマラソン"},
	{"Berlin Marathon", "ベルリンマラソン"},
	{"Boston Marathon", "ボストンマラソン"},
	{"Shanghai Marathon", "上海マラソン"},
	{"Maratón Valencia", "バレンシアマラソン"},
}

var cityNames = []titled{
	{"Bhubaneshwar", "ブバネーシュワル"},
	{"Bhubaneswar", "ブバネーシュワル"},
	{"Yangzhou", "揚州"},
	{"København", "コペンハーゲン"},
	{"Copenhagen", "コペンハーゲン"},
	{"Brasilia", "ブラジリア"},
	{"Brasília", "ブラジリア"},
	{"Budapest", "ブダペスト"},
	{"Bruxelles", "ブリュッセル"},
	{"Brussels", "ブリュッセル"},
	{"Chorzów", "ホジュフ"},
	{"London", "ロンドン"},
	{"Lausanne", "ローザンヌ"},
	{"Monaco", "モナコ"},
	{"Paris", "パリ"},
	{"Stockholm", "ストックホルム"},
	{"Oslo", "オスロ"},
	{"Rabat", "ラバト"},
	{"Roma", "ローマ"},
	{"Rome", "ローマ"},
	{"Xiamen", "アモイ"},
	{"Shanghai", "上海"},
	{"Shaoxing", "紹興"},
	{"Keqiao", "柯橋"},
	{"Doha", "ドーハ"},
	{"Valencia", "バレンシア"},
	{"New York City", "ニューヨーク"},
	{"New York", "ニューヨーク"},
	{"Amsterdam", "アムステルダム"},
	{"Chicago", "シカゴ"},
	{"Berlin", "ベルリン"},
	{"Zürich", "チューリッヒ"},
	{"Zurich", "チューリッヒ"},
	{"Sydney", "シドニー"},
	{"Boston", "ボストン"},
	{"Tokyo", "東京"},
	{"Osaka", "大阪"},
	{"Yokohama", "横浜"},
	{"Nagoya", "名古屋"},
	{"Sapporo", "札幌"},
	{"Fukuoka", "福岡"},
	{"Kobe", "神戸"},
	{"Kyoto", "京都"},
	{"Hiroshima", "広島"},
	{"Chiba", "千葉"},
	{"Sendai", "仙台"},
	{"Los Angeles", "ロサンゼルス"},
	{"San Francisco", "サンフランシスコ"},
	{"San Diego", "サンディエゴ"},
	{"San Jose", "サンホセ"},
	{"San Antonio", "サンアントニオ"},
	{"Philadelphia", "フィラデルフィア"},
	{"Houston", "ヒューストン"},
	{"Washington", "ワシントン"},
	{"Miami", "マイアミ"},
	{"Atlanta", "アトランタ"},
	{"Seattle", "シアトル"},
	{"Portland", "ポートランド"},
	{"Denver", "デンバー"},
	{"Dallas", "ダラス"},
	{"Austin", "オースティン"},
	{"Honolulu", "ホノルル"},
	{"Las Vegas", "ラスベガス"},
	{"Salt Lake City", "ソルトレークシティ"},
	{"Kansas City", "カンザスシティ"},
	{"New Orleans", "ニューオーリンズ"},
	{"Baltimore", "ボルチモア"},
	{"Detroit", "デトロイト"},
	{"Minneapolis", "ミネアポリス"},
	{"Phoenix", "フェニックス"},
	{"Indianapolis", "インディアナポリス"},
	{"Sacramento", "サクラメント"},
	{"Fayetteville", "フェイエットビル"},
	{"Stanford", "スタンフォード"},
	{"Des Moines", "デモイン"},
	{"St. Louis", "セントルイス"},
	{"St Louis", "セントルイス"},
	{"Saint Louis", "セントルイス"},
	{"Eugene", "ユージーン"},
	{"Madrid", "マドリード"},
	{"Barcelona", "バルセロナ"},
	{"Seville", "セビリア"},
	{"Sevilla", "セビリア"},
	{"Lisbon", "リスボン"},
	{"Lisboa", "リスボン"},
	{"Porto", "ポルト"},
	{"Milan", "ミラノ"},
	{"Milano", "ミラノ"},
	{"Florence", "フィレンツェ"},
	{"Firenze", "フィレンツェ"},
	{"Naples", "ナポリ"},
	{"Napoli", "ナポリ"},
	{"Turin", "トリノ"},
	{"Torino", "トリノ"},
	{"Bologna", "ボローニャ"},
	{"Genoa", "ジェノバ"},
	{"Venice", "ヴェネツィア"},
	{"Verona", "ヴェローナ"},
	{"Lyon", "リヨン"},
	{"Marseille", "マルセイユ"},
	{"Nice", "ニース"},
	{"Saint-Denis", "サン＝ドニ"},
	{"Geneva", "ジュネーブ"},
	{"Genève", "ジュネーブ"},
	{"Bern", "ベルン"},
	{"Munich", "ミュンヘン"},
	{"München", "ミュンヘン"},
	{"Hamburg", "ハンブルク"},
	{"Frankfurt", "フランクフルト"},
	{"Cologne", "ケルン"},
	{"Köln", "ケルン"},
	{"Stuttgart", "シュトゥットガルト"},
	{"Düsseldorf", "デュッセルドルフ"},
	{"Dusseldorf", "デュッセルドルフ"},
	{"Leipzig", "ライプツィヒ"},
	{"Vienna", "ウィーン"},
	{"Wien", "ウィーン"},
	{"Prague", "プラハ"},
	{"Praha", "プラハ"},
	{"Warsaw", "ワルシャワ"},
	{"Warszawa", "ワルシャワ"},
	{"Krakow", "クラクフ"},
	{"Kraków", "クラクフ"},
	{"Gdansk", "グダニスク"},
	{"Gdańsk", "グダニスク"},
	{"Rotterdam", "ロッテルダム"},
	{"Antwerp", "アントワープ"},
	{"Antwerpen", "アントワープ"},
	{"Gothenburg", "ヨーテボリ"},
	{"Göteborg", "ヨーテボリ"},
	{"Helsinki", "ヘルシンキ"},
	{"Athens", "アテネ"},
	{"Istanbul", "イスタンブール"},
	{"Ankara", "アンカラ"},
	{"Glasgow", "グラスゴー"},
	{"Birmingham", "バーミンガム"},
	{"Manchester", "マンチェスター"},
	{"Edinburgh", "エディンバラ"},
	{"Dublin", "ダブリン"},
	{"Cardiff", "カーディフ"},
	{"Gateshead", "ゲーツヘッド"},
	{"Silesia", "シロンスク"},
	{"Melbourne", "メルボルン"},
	{"Brisbane", "ブリスベン"},
	{"Perth", "パース"},
	{"Adelaide", "アデレード"},
	{"Gold Coast", "ゴールドコースト"},
	{"Canberra", "キャンベラ"},
	{"Auckland", "オークランド"},
	{"Wellington", "ウェリントン"},
	{"Christchurch", "クライストチャーチ"},
	{"Guangzhou", "広州"},
	{"Shenzhen", "深セン"},
	{"Nanjing", "南京"},
	{"Hangzhou", "杭州"},
	{"Chengdu", "成都"},
	{"Wuhan", "武漢"},
	{"Beijing", "北京"},
	{"Hong Kong", "香港"},
	{"Macau", "マカオ"},
	{"Taipei", "台北"},
	{"Kaohsiung", "高雄"},
	{"Seoul", "ソウル"},
	{"Busan", "釜山"},
	{"Incheon", "仁川"},
	{"Singapore", "シンガポール"},
	{"Bangkok", "バンコク"},
	{"Kuala Lumpur", "クアラルンプール"},
	{"Jakarta", "ジャカルタ"},
	{"Manila", "マニラ"},
	{"Hanoi", "ハノイ"},
	{"Ho Chi Minh City", "ホーチミン"},
	{"Ho Chi Minh", "ホーチミン"},
	{"New Delhi", "ニューデリー"},
	{"Delhi", "デリー"},
	{"Mumbai", "ムンバイ"},
	{"Bengaluru", "ベンガルール"},
	{"Bangalore", "バンガロール"},
	{"Chennai", "チェンナイ"},
	{"Kolkata", "コルカタ"},
	{"Hyderabad", "ハイデラバード"},
	{"Dubai", "ドバイ"},
	{"Abu Dhabi", "アブダビ"},
	{"Riyadh", "リヤド"},
	{"Jeddah", "ジェッダ"},
	{"Kuwait City", "クウェート"},
	{"Manama", "マナーマ"},
	{"Muscat", "マスカット"},
	{"Tel Aviv", "テルアビブ"},
	{"Jerusalem", "エルサレム"},
	{"Cairo", "カイロ"},
	{"Alexandria", "アレクサンドリア"},
	{"Casablanca", "カサブランカ"},
	{"Marrakech", "マラケシュ"},
	{"Marrakesh", "マラケシュ"},
	{"Tunis", "チュニス"},
	{"Algiers", "アルジェ"},
	{"Nairobi", "ナイロビ"},
	{"Mombasa", "モンバサ"},
	{"Addis Ababa", "アディスアベバ"},
	{"Kampala", "カンパラ"},
	{"Kigali", "キガリ"},
	{"Dar es Salaam", "ダルエスサラーム"},
	{"Lagos", "ラゴス"},
	{"Accra", "アクラ"},
	{"Dakar", "ダカール"},
	{"Abidjan", "アビジャン"},
	{"Johannesburg", "ヨハネスブルグ"},
	{"Pretoria", "プレトリア"},
	{"Cape Town", "ケープタウン"},
	{"Durban", "ダーバン"},
	{"Gaborone", "ハボローネ"},
	{"Windhoek", "ウィントフック"},
	{"Harare", "ハラレ"},
	{"Lusaka", "ルサカ"},
	{"Port Louis", "ポートルイス"},
	{"Toronto", "トロント"},
	{"Montreal", "モントリオール"},
	{"Vancouver", "バンクーバー"},
	{"Ottawa", "オタワ"},
	{"Calgary", "カルガリー"},
	{"Edmonton", "エドモントン"},
	{"Mexico City", "メキシコシティ"},
	{"Guadalajara", "グアダラハラ"},
	{"Havana", "ハバナ"},
	{"Nassau", "ナッソー"},
	{"Kingston", "キングストン"},
	{"Santo Domingo", "サントドミンゴ"},
	{"San Juan", "サンフアン"},
	{"Port of Spain", "ポートオブスペイン"},
	{"São Paulo", "サンパウロ"},
	{"Rio de Janeiro", "リオデジャネイロ"},
	{"Buenos Aires", "ブエノスアイレス"},
	{"Santiago", "サンティアゴ"},
	{"Lima", "リマ"},
	{"Bogotá", "ボゴタ"},
	{"Bogota", "ボゴタ"},
	{"Quito", "キト"},
	{"Medellín", "メデジン"},
	{"Medellin", "メデジン"},
	{"Caracas", "カラカス"},
	{"Montevideo", "モンテビデオ"},
	{"La Paz", "ラパス"},
	{"Panama City", "パナマシティ"},
	{"Belgrade", "ベオグラード"},
	{"Bucharest", "ブカレスト"},
	{"Sofia", "ソフィア"},
	{"Zagreb", "ザグレブ"},
	{"Ljubljana", "リュブリャナ"},
	{"Bratislava", "ブラチスラバ"},
	{"Moscow", "モスクワ"},
	{"Saint Petersburg", "サンクトペテルブルク"},
	{"St. Petersburg", "サンクトペテルブルク"},
	{"St Petersburg", "サンクトペテルブルク"},
	{"Kyiv", "キーウ"},
	{"Kiev", "キーウ"},
	{"Minsk", "ミンスク"},
	{"Sochi", "ソチ"},
	{"Tbilisi", "トビリシ"},
	{"Yerevan", "エレバン"},
	{"Baku", "バクー"},
	{"Almaty", "アルマトイ"},
	{"Tashkent", "タシケント"},
	{"Astana", "アスタナ"},
	{"Ulaanbaatar", "ウランバートル"},
	{"The Hague", "ハーグ"},
	{"Den Haag", "ハーグ"},
}

var countries = map[string]string{
	"AFG": "アフガニスタン", "ALB": "アルバニア", "ALG": "アルジェリア", "AND": "アンドラ", "ANG": "アンゴラ",
	"ANT": "アンティグア・バーブーダ", "ARG": "アルゼンチン", "ARM": "アルメニア", "ARU": "アルバ", "ASA": "アメリカ領サモア",
	"AUS": "オーストラリア", "AUT": "オーストリア", "AZE": "アゼルバイジャン", "BAH": "バハマ", "BAN": "バングラデシュ",
	"BAR": "バルバドス", "BDI": "ブルンジ", "BEL": "ベルギー", "BEN": "ベナン", "BER": "バミューダ",
	"BHU": "ブータン", "BIH": "ボスニア・ヘルツェゴビナ", "BIZ": "ベリーズ", "BLR": "ベラルーシ", "BOL": "ボリビア",
	"BOT": "ボツワナ", "BRA": "ブラジル", "BRN": "バーレーン", "BRU": "ブルネイ", "BUL": "ブルガリア",
	"BUR": "ブルキナファソ", "CAF": "中央アフリカ", "CAM": "カンボジア", "CAN": "カナダ", "CAY": "ケイマン諸島",
	"CGO": "コンゴ共和国", "CHA": "チャド", "CHI": "チリ", "CHN": "中国", "CIV": "コートジボワール",
	"CMR": "カメルーン", "COD": "コンゴ民主共和国", "COK": "クック諸島", "COL": "コロンビア", "COM": "コモロ",
	"CPV": "カーボベルデ", "CRC": "コスタリカ", "CRO": "クロアチア", "CUB": "キューバ", "CYP": "キプロス",
	"CZE": "チェコ", "DEN": "デンマーク", "DJI": "ジブチ", "DMA": "ドミニカ国", "DOM": "ドミニカ共和国",
	"ECU": "エクアドル", "EGY": "エジプト", "ERI": "エリトリア", "ESA": "エルサルバドル", "ESP": "スペイン",
	"EST": "エストニア", "ETH": "エチオピア", "FIJ": "フィジー", "FIN": "フィンランド", "FRA": "フランス",
	"FSM": "ミクロネシア", "GAB": "ガボン", "GAM": "ガンビア", "GBR": "イギリス", "GBS": "ギニアビサウ",
	"GEO": "ジョージア", "GEQ": "赤道ギニア", "GER": "ドイツ", "GHA": "ガーナ", "GRE": "ギリシャ",
	"GRN": "グレナダ", "GUA": "グアテマラ", "GUI": "ギニア", "GUM": "グアム", "GUY": "ガイアナ",
	"HAI": "ハイチ", "HKG": "香港", "HON": "ホンジュラス", "HUN": "ハンガリー", "INA": "インドネシア",
	"IND": "インド", "IRI": "イラン", "IRL": "アイルランド", "IRQ": "イラク", "ISL": "アイスランド",
	"ISR": "イスラエル", "ISV": "米領バージン諸島", "ITA": "イタリア", "IVB": "英領バージン諸島", "JAM": "ジャマイカ",
	"JOR": "ヨルダン", "JPN": "日本", "KAZ": "カザフスタン", "KEN": "ケニア", "KGZ": "キルギス",
	"KIR": "キリバス", "KOR": "韓国", "KOS": "コソボ", "KSA": "サウジアラビア", "KUW": "クウェート",
	"LAO": "ラオス", "LAT": "ラトビア", "LBA": "リビア", "LBN": "レバノン", "LBR": "リベリア",
	"LCA": "セントルシア", "LES": "レソト", "LIE": "リヒテンシュタイン", "LTU": "リトアニア", "LUX": "ルクセンブルク",
	"MAD": "マダガスカル", "MAR": "モロッコ", "MAS": "マレーシア", "MAW": "マラウイ", "MDA": "モルドバ",
	"MDV": "モルディブ", "MEX": "メキシコ", "MGL": "モンゴル", "MHL": "マーシャル諸島", "MKD": "北マケドニア",
	"MLI": "マリ", "MLT": "マルタ", "MNE": "モンテネグロ", "MON": "モナコ", "MOZ": "モザンビーク",
	"MRI": "モーリシャス", "MTN": "モーリタニア", "MYA": "ミャンマー", "NAM": "ナミビア", "NCA": "ニカラグア",
	"NED": "オランダ", "NEP": "ネパール", "NGR": "ナイジェリア", "NIG": "ニジェール", "NOR": "ノルウェー",
	"NRU": "ナウル", "NZL": "ニュージーランド", "OMA": "オマーン", "PAK": "パキスタン", "PAN": "パナマ",
	"PAR": "パラグアイ", "PER": "ペルー", "PHI": "フィリピン", "PLE": "パレスチナ", "PLW": "パラオ",
	"PNG": "パプアニューギニア", "POL": "ポーランド", "POR": "ポルトガル", "PRK": "北朝鮮", "PUR": "プエルトリコ",
	"QAT": "カタール", "ROU": "ルーマニア", "RSA": "南アフリカ", "RUS": "ロシア", "RWA": "ルワンダ",
	"SAM": "サモア", "SEN": "セネガル", "SEY": "セーシェル", "SGP": "シンガポール", "SKN": "セントクリストファー・ネイビス",
	"SLE": "シエラレオネ", "SLO": "スロベニア", "SMR": "サンマリノ", "SOL": "ソロモン諸島", "SOM": "ソマリア",
	"SRB": "セルビア", "SRI": "スリランカ", "SSD": "南スーダン", "STP": "サントメ・プリンシペ", "SUD": "スーダン",
	"SUI": "スイス", "SUR": "スリナム", "SVK": "スロバキア", "SWE": "スウェーデン", "SWZ": "エスワティニ",
	"SYR": "シリア", "TAN": "タンザニア", "TGA": "トンガ", "THA": "タイ", "TJK": "タジキスタン",
	"TKM": "トルクメニスタン", "TLS": "東ティモール", "TOG": "トーゴ", "TPE": "チャイニーズ・タイペイ", "TTO": "トリニダード・トバゴ",
	"TUN": "チュニジア", "TUR": "トルコ", "TUV": "ツバル", "UAE": "アラブ首長国連邦", "UGA": "ウガンダ",
	"UKR": "ウクライナ", "URU": "ウルグアイ", "USA": "アメリカ", "UZB": "ウズベキスタン", "VAN": "バヌアツ",
	"VEN": "ベネズエラ", "VIE": "ベトナム", "VIN": "セントビンセント", "YEM": "イエメン", "ZAM": "ザンビア",
	"ZIM": "ジンバブエ",
}
