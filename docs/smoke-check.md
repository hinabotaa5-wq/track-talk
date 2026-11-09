# デプロイ後の読み取り専用スモークチェック

`deploy/smoke/readonly-smoke.sh` は、デプロイの後に本番の API と Web を外から叩いて、申込みまわりの基本の挙動が変わっていないかを確かめるスクリプトです。

## やらないこと

- 本番への書き込み（申込みの作成・決済・フラグの変更）はしません。
- ログインせず、認証情報も使いません。
- 送るのは GET、CORS のプリフライト（OPTIONS）、未認証で中身が空の POST（認証で 401 になり、申込みは作られない）だけです。

## 実行

```sh
bash deploy/smoke/readonly-smoke.sh
```

| 環境変数 | 既定 | 意味 |
| --- | --- | --- |
| `WEB_URL` | `https://track-talk.netlify.app` | Web の URL |
| `API_URL` | 空 | API の URL。空なら Web の CSP（connect-src）に載っている API の URL を使う |
| `EXPECT_COACH_LIST` | `off` | コーチ一覧のフラグの想定。`off` なら `/v1/coaches` は 404、`on` なら 200 |

すべて合格なら終了コード 0、1件でも不合格なら 1、URL などの前提が揃わなければ 2 で終わります。接続できなかった項目は不合格として数えます（応答がないことを合格にしません）。

## 確かめる項目

1. `GET /health` が 200 で `{"status":"ok"}`
2. `GET /v1/coaches` がコーチ一覧のフラグの想定どおり（閉じていれば 404）
3. 未認証で `GET /v1/coaching/nominees`・`POST /v1/coaching/requests`・`GET /v1/me`・`GET /v1/coach/me`・`GET /v1/admin/coaching/payments` が 401
4. 許可していない Origin からの CORS プリフライトに `Access-Control-Allow-Origin` が付かない
5. 特商法ページ（`/legal/commerce`）にフォーム添削 1,500円（税込）と追加相談 600円（税込）の記載がある

同じ挙動は API の回帰テスト（`apps/api/internal/httpx/coaching_hardening_test.go`）でも固定しています。スクリプトは、デプロイした本番の設定（フラグ・CORS の許可先）でもそうなっているかを見るためのものです。
