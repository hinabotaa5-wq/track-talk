package httpx_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestReservedProfileNicknames(t *testing.T) {
	h := setup(t)
	id := uuid.New()
	tok := tokenWithProfile(t, id, "公式アカウント", true)
	me := doJSON(t, h, http.MethodGet, "/v1/me", tok, nil)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"nickname":null`) ||
		!strings.Contains(me.Body.String(), `"nickname_public":false`) {
		t.Fatalf("reserved identity nickname was imported: %d %s", me.Code, me.Body)
	}

	patch := func(t *testing.T, auth, nickname string, status int, fragment string) {
		t.Helper()
		rec := doJSON(t, h, http.MethodPatch, "/v1/me", auth, map[string]any{
			"nickname": nickname, "nickname_public": true, "event": "100m",
		})
		if rec.Code != status || !strings.Contains(rec.Body.String(), fragment) {
			t.Fatalf("nickname %q: %d %s", nickname, rec.Code, rec.Body)
		}
	}
	for _, name := range []string{
		"管理者", "副管理者", "運営チーム", "公式アカウント", "かんりしゃ", "管 理 者", "ａｄｍｉｎ",
		"ADMIN_taro", "サポート担当", "事務局の中の人", "TrackTalk", "トラックトーク",
		"公式戦運営", "サポーター管理者", "ｱﾄﾞﾐﾝ", "a_d.m-i・n", "a\u200bdmin", "公式記録",
	} {
		patch(t, tok, name, http.StatusUnprocessableEntity, `"code":"nickname_reserved"`)
	}
	other := token(t, uuid.New())
	for _, name := range []string{
		"たなかたろう", "陸上たろう", "管理栄養士たろう", "モデラーのたろう", "runnerたろう",
		"サポーターたろう", "公式戦がんばる", "サポーターズ", "supporter", "supporters",
	} {
		patch(t, other, name, http.StatusOK, `"nickname":"`+name+`"`)
	}
	patch(t, other, "管理者", http.StatusUnprocessableEntity, `"code":"nickname_reserved"`)

	execSQLArgs(t, `UPDATE profiles SET nickname = '運営', nickname_public = false WHERE id = $1`, id)
	patch(t, tok, "運営", http.StatusOK, `"nickname_public":true`)
	patch(t, tok, "運営者", http.StatusUnprocessableEntity, `"code":"nickname_reserved"`)

	admin := adminToken(t, h)
	patch(t, admin, "運営", http.StatusOK, `"nickname":"運営"`)
	_, coach := onDutyCoach(t, h, admin)
	patch(t, coach, "運営", http.StatusUnprocessableEntity, `"code":"nickname_reserved"`)
}
