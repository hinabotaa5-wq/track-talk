#!/usr/bin/env bash
# デプロイ後の読み取り専用スモークチェック（本体 #474）。
#
# 本番に何も書き込まない。ログイン・申込みの作成・決済・フラグの変更はしない。認証情報も使わない。
# 送るのは GET と CORS のプリフライト（OPTIONS）と、未認証で中身が空の POST（認証で 401 になり、申込みは作られない）だけ。
#
# 使い方:
#   bash deploy/smoke/readonly-smoke.sh
#   WEB_URL=https://track-talk.netlify.app API_URL=https://<API の URL> EXPECT_COACH_LIST=off bash deploy/smoke/readonly-smoke.sh
#
# 環境変数:
#   WEB_URL            Web の URL。既定は https://track-talk.netlify.app
#   API_URL            API の URL。空なら、Web の CSP（connect-src）に載っている track-talk-api の URL を使う。
#   EXPECT_COACH_LIST  コーチ一覧のフラグの想定。off（既定）なら /v1/coaches は 404、on なら 200。
#
# 終了コード: すべて合格なら 0、1件でも不合格なら 1、前提（URL など）が揃わなければ 2。
set -uo pipefail

WEB_URL="${WEB_URL:-https://track-talk.netlify.app}"
WEB_URL="${WEB_URL%/}"
API_URL="${API_URL:-}"
EXPECT_COACH_LIST="${EXPECT_COACH_LIST:-off}"
EVIL_ORIGIN="https://smoke-check.invalid"
CURL=(curl -sS --max-time 20 --proto '=https,http' -A "track-talk-readonly-smoke")

pass=0
fail=0
ok() { pass=$((pass + 1)); printf 'PASS  %s\n' "$1"; }
ng() { fail=$((fail + 1)); printf 'FAIL  %s（%s）\n' "$1" "$2"; }

# status <method> <url> [curl の追加引数...] : HTTP ステータスだけを返す（リダイレクトは追わない）。
status() {
	local method="$1" url="$2"
	shift 2
	local code
	code=$("${CURL[@]}" -o /dev/null -w '%{http_code}' -X "$method" "$@" "$url" 2>/dev/null) || true
	echo "${code:-000}"
}

expect_status() {
	local name="$1" want="$2" method="$3" url="$4"
	shift 4
	local got
	got=$(status "$method" "$url" "$@")
	if [[ "$got" == "$want" ]]; then ok "$name → $got"; else ng "$name" "期待 $want、実際 $got"; fi
}

case "$EXPECT_COACH_LIST" in
on) coaches_want=200 ;;
off) coaches_want=404 ;;
*)
	echo "EXPECT_COACH_LIST は on か off を指定してください" >&2
	exit 2
	;;
esac

if [[ -z "$API_URL" ]]; then
	API_URL=$("${CURL[@]}" -I "$WEB_URL/" 2>/dev/null | tr -d '\r' | grep -i '^content-security-policy:' |
		grep -oE 'https://track-talk-api-[a-z0-9.-]+\.run\.app' | head -n1)
	if [[ -z "$API_URL" ]]; then
		echo "API_URL が空で、Web の CSP からも見つかりませんでした。API_URL を指定してください" >&2
		exit 2
	fi
fi
API_URL="${API_URL%/}"
echo "Web: $WEB_URL"
echo "API: $API_URL"
echo "コーチ一覧の想定: $EXPECT_COACH_LIST"
echo

# 1. API の生存確認。
health=$("${CURL[@]}" -w '\n%{http_code}' "$API_URL/health" 2>/dev/null || true)
if [[ "$(tail -n1 <<<"$health")" == "200" ]] && grep -q '"status":"ok"' <<<"$health"; then
	ok "GET /health → 200 {\"status\":\"ok\"}"
else
	ng "GET /health" "$(tr '\n' ' ' <<<"$health")"
fi

# 2. コーチ一覧はフラグの状態どおり。
expect_status "GET /v1/coaches（一覧 $EXPECT_COACH_LIST）" "$coaches_want" GET "$API_URL/v1/coaches"

# 3. 未認証の申込みまわりは 401（申込みは作られない）。
expect_status "GET /v1/coaching/nominees（未認証）" 401 GET "$API_URL/v1/coaching/nominees?event=100m"
expect_status "POST /v1/coaching/requests（未認証・空の本文）" 401 POST "$API_URL/v1/coaching/requests" \
	-H 'Content-Type: application/json' --data '{}'
expect_status "GET /v1/me（未認証）" 401 GET "$API_URL/v1/me"
expect_status "GET /v1/coach/me（未認証）" 401 GET "$API_URL/v1/coach/me"
expect_status "GET /v1/admin/coaching/payments（未認証）" 401 GET "$API_URL/v1/admin/coaching/payments"

# 4. 許可していない Origin には Access-Control-Allow-Origin を返さない。
# 応答そのものが返らなかったとき（接続できない等）は、ヘッダーが無くても合格にしない。
preflight=$("${CURL[@]}" -o /dev/null -D - -X OPTIONS "$API_URL/v1/coaching/requests" \
	-H "Origin: $EVIL_ORIGIN" -H 'Access-Control-Request-Method: POST' \
	-H 'Access-Control-Request-Headers: authorization,content-type' 2>/dev/null | tr -d '\r' || true)
acao=$(grep -i '^access-control-allow-origin:' <<<"$preflight" || true)
if ! grep -qE '^HTTP/[0-9.]+ [0-9]{3}' <<<"$preflight"; then
	ng "CORS プリフライト（許可外の Origin）" "応答なし"
elif [[ -n "$acao" ]]; then
	ng "CORS プリフライト（許可外の Origin）" "$acao"
else
	ok "CORS プリフライト（許可外の Origin）→ Access-Control-Allow-Origin なし"
fi

# 5. 特商法ページの料金（フォーム添削 1,500円・追加相談 600円、いずれも税込）。
commerce=$("${CURL[@]}" -w '\n%{http_code}' "$WEB_URL/legal/commerce" 2>/dev/null | sed 's/<!-- -->//g' || true)
if [[ "$(tail -n1 <<<"$commerce")" != "200" ]]; then
	ng "GET /legal/commerce" "ステータス $(tail -n1 <<<"$commerce")"
else
	for price in '1,500円（税込）' '600円（税込）'; do
		if grep -q "$price" <<<"$commerce"; then ok "/legal/commerce に「$price」"; else ng "/legal/commerce の料金" "「$price」が見つからない"; fi
	done
fi

echo
echo "合格 $pass 件・不合格 $fail 件"
[[ $fail -eq 0 ]]
