package httpx

import (
	"net/url"
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
	if utf8.RuneCountInString(v) > 120 {
		return "120文字以内で入力してください"
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

func PostBody(v string) string {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > 10000 {
		return "10000文字以内で入力してください"
	}
	return ""
}

func Kind(v string) string {
	switch v {
	case "text", "youtube", "video":
		return ""
	default:
		return "種類が不正です"
	}
}

func YouTubeVideoID(v string) string {
	if len(v) != 11 {
		return "動画IDが不正です"
	}
	for _, r := range v {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return "動画IDが不正です"
		}
	}
	return ""
}

func HTTPSURL(v string) string {
	if len(v) == 0 || len(v) > 500 {
		return "サムネイルのURLが不正です"
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "サムネイルのURLが不正です"
	}
	return ""
}

func VideoPublicID(v string) string {
	n := utf8.RuneCountInString(v)
	if n < 1 || n > 255 || strings.Contains(v, "..") {
		return "動画IDが不正です"
	}
	for _, r := range v {
		if r == ' ' || r == '\n' || r == '\t' || r == '?' || r == '#' {
			return "動画IDが不正です"
		}
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
