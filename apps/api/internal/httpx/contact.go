package httpx

import (
	"regexp"

	"golang.org/x/text/width"
)

// 添削の返答で、コーチと利用者が直接やりとりするための連絡先らしき文字列を見つける。
// 送る前にその場で止めて直してもらう（online-coaching/design-review.md の「2-3」）。
// 全角の英数字は半角にそろえてから調べる。見落としより止めすぎを選ぶが、陸上の用語では止めない。

var contactPatterns = []*regexp.Regexp{
	// URL とドメイン
	regexp.MustCompile(`(?i)https?://|www\.`),
	regexp.MustCompile(`(?i)\b[a-z0-9-]+\.(com|net|org|jp|io|me|co|app|link|ly|gg|tv)\b`),
	// メールアドレス
	regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`),
	// SNS のアカウント名（@ のあとに英数字）
	regexp.MustCompile(`(?i)@[a-z0-9_.]{3,}`),
	// 電話番号（区切りを含めて10桁以上の数字）
	regexp.MustCompile(`(?:\d[\s\-–ー()]*){10,}`),
	// SNS やメッセージアプリの名前と、ID のやりとり
	regexp.MustCompile(`(?i)\bline\b|instagram|インスタ|twitter|ツイッター|tiktok|ティックトック|discord|ディスコード|カカオ|kakao|\bDM\b|メアド|メールアドレス|電話番号|連絡先|\bID\s*[:：は]`),
}

// looksLikeContact は、文章に連絡先らしき文字列があれば true を返す。
func looksLikeContact(s string) bool {
	folded := width.Fold.String(s)
	for _, re := range contactPatterns {
		if re.MatchString(folded) {
			return true
		}
	}
	return false
}
