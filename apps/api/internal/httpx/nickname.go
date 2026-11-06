package httpx

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const reservedNicknameMessage = "このニックネームは使用できません。運営や公式を思わせる名前は使えません。"

var nicknameExceptions = strings.NewReplacer("サポーター", "", "公式戦", "", "supporters", "", "supporter", "")

var reservedNicknameWords = [...]string{
	"管理者", "管理人", "かんりしゃ", "カンリシャ", "かんりにん",
	"運営", "うんえい", "ウンエイ", "事務局", "じむきょく", "公式", "こうしき", "コウシキ",
	"スタッフ", "サポート", "モデレーター", "アドミン", "トラックトーク",
	"admin", "moderator", "staff", "support", "official", "system", "tracktalk",
}

func isReservedNickname(value string) bool {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.Is(unicode.Cf, r) ||
			unicode.Is(unicode.Variation_Selector, r) || r == '\u034f' {
			return -1
		}
		return r
	}, strings.ToLower(norm.NFKC.String(value)))

	// 区切りの除去で隣り合った濁点などを再び合成する。
	normalized = nicknameExceptions.Replace(norm.NFC.String(normalized))
	for _, word := range reservedNicknameWords {
		if strings.Contains(normalized, word) {
			return true
		}
	}
	return false
}
