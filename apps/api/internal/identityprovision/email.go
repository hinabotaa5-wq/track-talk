// Package identityprovision は、合成データ専用のプロビジョニング（provision／verify／cleanup）と、
// 合成コーチでの通し確認（walkthrough）を行う Cloud Run Job の中身（ops #63）。
//
// 実行体だけが資格情報（金庫 DB のアプリ用ロール・本体の読み取りロール・金庫 Auth の管理用キー）を持ち、
// 起動する人には件数・真偽・ステータスだけを返す。合成パスワード・TOTP の設定キー・コードは実行中の
// メモリにだけ置き、出力・ログ・どこへの保存もしない。
package identityprovision

import (
	"errors"
	"regexp"
	"strings"
)

// 合成メールは、運営者が管理する受信箱のアドレス（base）の @ の前に「+synthetic-<tag>」を足した形だけを認める。
// base は公開リポジトリに書かず、ジョブの設定値として渡す。
var (
	tagPattern  = regexp.MustCompile(`^[a-z0-9]{1,32}$`)
	basePattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}@[a-z0-9-]+(\.[a-z0-9-]+)+$`)
)

var (
	ErrInvalidBase = errors.New("synthetic_base_invalid")
	ErrInvalidTag  = errors.New("synthetic_tag_invalid")
)

// SyntheticEmail は base と tag から合成メールを作る。
func SyntheticEmail(base, tag string) (string, error) {
	base = strings.ToLower(strings.TrimSpace(base))
	if !basePattern.MatchString(base) {
		return "", ErrInvalidBase
	}
	if !tagPattern.MatchString(tag) {
		return "", ErrInvalidTag
	}
	local, domain, _ := strings.Cut(base, "@")
	return local + "+synthetic-" + tag + "@" + domain, nil
}

// IsSynthetic は、email が base から作った合成メールの形かを返す（大文字小文字は区別しない）。
func IsSynthetic(base, email string) bool {
	base = strings.ToLower(strings.TrimSpace(base))
	email = strings.ToLower(strings.TrimSpace(email))
	if !basePattern.MatchString(base) {
		return false
	}
	local, domain, _ := strings.Cut(base, "@")
	rest, ok := strings.CutPrefix(email, local+"+synthetic-")
	if !ok {
		return false
	}
	tag, ok := strings.CutSuffix(rest, "@"+domain)
	return ok && tagPattern.MatchString(tag)
}
