package httpx

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

// 添削の返答で、コーチと利用者が直接やりとりするための連絡先らしき文字列を見つける。
// 送る前にその場で止めて直してもらう（online-coaching/design-review.md の「2-3」）。
// 字形の違い(全角・飾り文字・丸数字・漢数字)と見えない文字をそろえてから調べる。
// 見落としより止めすぎを選ぶが、陸上の用語(スタートライン、ラップの「12.1 12.3」など)では止めない。
// 「at」「dot」と書く、何通かに分けて送るといった言い換えまでは追わない(通報と運営の確認で拾う)。

var contactPatterns = []*regexp.Regexp{
	// URL とドメイン
	regexp.MustCompile(`(?i)https?://|www\.`),
	regexp.MustCompile(`(?i)\b[a-z0-9-]+\.(com|net|org|jp|io|me|co|app|link|ly|gg|tv|ee|dev|xyz|info|page|site|blog)\b`),
	// メールアドレス
	regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`),
	// SNS のアカウント名（@ のあとに英数字）
	regexp.MustCompile(`(?i)@[a-z0-9_.]{3,}`),
	// SNS やメッセージアプリの名前と、ID のやりとり
	regexp.MustCompile(`(?i)\bline\b|instagram|\binsta\b|インスタ|twitter|ツイッター|tiktok|ティックトック|discord|ディスコード|カカオ|kakao|facebook|フェイスブック|threads|スレッズ|\bDM\b|メアド|メールアドレス|電話番号|連絡先|\bID\s*[:：は]`),
	// 「ライン」は陸上の用語でもあるので、交換や ID と一緒のときと、ひらがなのときだけ止める。
	regexp.MustCompile(`らいん|ライン\s*(交換|教え|追加|やって|で話|でやり|ID|id)`),
}

// looksLikeContact は、文章に連絡先らしき文字列があれば true を返す。
func looksLikeContact(s string) bool {
	folded := normalizeForContact(s)
	if looksLikePhone(folded) {
		return true
	}
	for _, re := range contactPatterns {
		if re.MatchString(folded) {
			return true
		}
	}
	return false
}

// 区切りでつながった数字の並び。ラップを「.」で書くので「.」は区切りに入れない。
var digitRun = regexp.MustCompile(`\d(?:[\s\-–ー()_/・]*\d)*`)

// looksLikePhone は、電話番号らしい数字の並びがあれば true を返す。
// 区切りのない10桁以上の数字と、0 か 81 で始まる10〜13桁で、区切りが3つまで(090-1234-5678、+81 90 1234 5678)
// か1桁ずつ空けたものを電話番号とみなす。ラップ(68 68 67 67 66)や年の並び(2019 2020 2021)は止めない。
func looksLikePhone(s string) bool {
	for _, run := range digitRun.FindAllString(s, -1) {
		groups := strings.FieldsFunc(run, func(r rune) bool { return r < '0' || r > '9' })
		digits := 0
		single := true
		for _, g := range groups {
			digits += len(g)
			single = single && len(g) == 1
		}
		if len(groups) == 1 && digits >= 10 {
			return true
		}
		first := groups[0]
		if digits >= 10 && digits <= 13 && (first[0] == '0' || strings.HasPrefix(first, "81")) && (len(groups) <= 4 || single) {
			return true
		}
	}
	return false
}

var kanjiDigits = strings.NewReplacer("〇", "0", "零", "0", "一", "1", "二", "2", "三", "3", "四", "4", "五", "5", "六", "6", "七", "7", "八", "8", "九", "9")

// normalizeForContact は、NFKC で字形をそろえ(飾り文字・丸数字・全角)、見えない文字と絵文字の飾りを除き、
// 漢数字を数字にする。漢数字が10字以上続く普通の文はないので、電話番号の判定にしか効かない。
func normalizeForContact(s string) string {
	s = norm.NFKC.String(width.Fold.String(s))
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Variation_Selector, r) {
			return -1
		}
		return r
	}, s)
	return kanjiDigits.Replace(s)
}
