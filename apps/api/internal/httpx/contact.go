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
// 日本語入力の全角ハイフン(−)やダッシュ(‐〜―)、波線も区切りにする。
var digitRun = regexp.MustCompile(`\d(?:[\s\-\x{2010}-\x{2015}\x{2212}ー~〜()_/・]*\d)*`)

// looksLikePhone は、電話番号らしい数字の並びがあれば true を返す。
// 区切りのない10桁以上の数字か、0 か 81 で始まる連続した4つまでの区切りで10〜13桁になり、
// 4桁以上の区切りを含むもの(090-1234-5678、+81 (0)90 1234 5678)か、1桁ずつ空けたものを電話番号とみなす。
// ラップ(68 68 67 67 66)や年の並び(2019 2020 2021)、日付(08/12 08/19)は止めない。
func looksLikePhone(s string) bool {
	for _, run := range digitRun.FindAllString(s, -1) {
		groups := strings.FieldsFunc(run, func(r rune) bool { return r < '0' || r > '9' })
		single := true
		for _, g := range groups {
			if len(g) >= 10 {
				return true
			}
			single = single && len(g) == 1
		}
		if single && len(groups) >= 10 && groups[0] == "0" {
			return true
		}
		for i, g := range groups {
			if g[0] != '0' && !strings.HasPrefix(g, "81") {
				continue
			}
			digits, long := 0, false
			for j := i; j < len(groups) && j < i+4; j++ {
				digits += len(groups[j])
				long = long || len(groups[j]) >= 4
				if digits >= 10 && digits <= 13 && long {
					return true
				}
			}
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
