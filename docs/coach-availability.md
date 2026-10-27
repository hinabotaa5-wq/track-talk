# コーチの空き日カレンダー

承認済みコーチが `/coach` で、今日から21日分の空き状況と30文字以内のメモを保存します。カレンダーはコーチ共通です。日本時間で日付を扱い、過去の日は取得・入力の対象から外します。

対面指導・大会サポートの公開プランがある場合に、公開プロフィールと該当プランの詳細にカレンダーを表示します。動画添削の詳細には表示しません。申し込み時の希望日は任意で最大3日です。○・△の日だけを選べ、申し込みの保存時にも現在の空き状況を確認します。その後カレンダーを変更しても、申し込みに保存した希望日は変わりません。

希望日は未確定の相談材料です。空き枠の確保や二重予約防止は行いません。日時はメッセージで相談し、対面指導には既存の提案・確定機能を使います。機能の公開フラグは変更していません。

## API

- `GET /v1/coaches/{id}/availability`：公開取得。対象プランがない・未承認・誓約未完了の場合は `days: []`。
- `GET /v1/coach/me/availability`：本人の入力済みデータ。承認と二段階認証の既存条件を適用します。
- `PUT /v1/coach/me/availability`：本人の空き状況を一括保存。送った日のみ更新し、`status: ""` でその日を未設定に戻します。本文は `{"days":[{"date":"2026-10-28","status":"available","note":"午後のみ"}]}`。状態は `available`、`consult`、`unavailable`、空文字。対象外の日・重複・不正な状態・30文字を超えるメモは422です。
- `POST /v1/coaching/contracts`：任意の `preferred_dates: ["YYYY-MM-DD"]` を受け付け、契約の取得・一覧にも同じ形式で返します。対象サービス・日付範囲・重複・上限・○△を検証します。

マイグレーションは `000115`（up/down）、SQLの生成は `make sqlc` です。

## 検証

```sh
make db
make migrate
make test-api
make test-web
```

ブラウザテストは、テスト用認証・API応答を使い、実際の画面で保存・希望日送信・エラー後の再選択・公開プロフィールの表示条件・390px幅での横はみ出しを確認します。

```sh
cd apps/web
NEXT_PUBLIC_COACHING_ENABLED=true \
NEXT_PUBLIC_SUPABASE_URL=http://127.0.0.1:54321 \
NEXT_PUBLIC_SUPABASE_ANON_KEY=availability-preview npm run dev
# 別のターミナルで実行します。
PLAYWRIGHT_COACHING_ENABLED=1 npx playwright test e2e/coach-availability.spec.ts --workers=1
```

`PLAYWRIGHT_SCREENSHOTS=1` を付けると `docs/screenshots/coach-availability/` にサンプルデータの画面を保存します。既に起動した別ポートのプレビューを使う場合は、`PLAYWRIGHT_SKIP_WEBSERVER=1` と `PLAYWRIGHT_BASE_URL` を指定します。
