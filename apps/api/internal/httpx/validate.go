package httpx

import (
	"strings"
	"unicode/utf8"
)

func Nickname(v string) string {
	v = strings.TrimSpace(v)
	if n := utf8.RuneCountInString(v); n < 2 || n > 30 {
		return "2〜30文字で入力してください"
	}
	for _, r := range v {
		if r == ' ' || r == '\n' || r == '\t' {
			return "空白は使えません"
		}
	}
	return ""
}

func Bio(v string) string {
	if utf8.RuneCountInString(v) > 500 {
		return "500文字以内で入力してください"
	}
	return ""
}

func Title(v string) string {
	v = strings.TrimSpace(v)
	n := utf8.RuneCountInString(v)
	if n < 1 || n > 120 {
		return "1〜120文字で入力してください"
	}
	return ""
}

func Body(v string, max int) string {
	v = strings.TrimSpace(v)
	n := utf8.RuneCountInString(v)
	if n < 1 || n > max {
		return "入力文字数を確認してください"
	}
	return ""
}

func Category(v string) string {
	switch v {
	case "race_impression", "record_discussion", "athlete_discussion", "general":
		return ""
	default:
		return "カテゴリが不正です"
	}
}

func ReportReason(v string) string {
	switch v {
	case "spam", "harassment", "misinfo", "other":
		return ""
	default:
		return "理由が不正です"
	}
}

func TargetType(v string) string {
	switch v {
	case "post", "comment":
		return ""
	default:
		return "対象が不正です"
	}
}

func ClampPage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 50 {
		perPage = 50
	}
	return page, perPage
}
