# 公開チェックリスト

料金・無料枠・商用条件は変動するため、公開直前に各公式ドキュメントを確認してください。

## Supabase

- プロジェクトを作成し、Auth のメール確認と Redirect URLs にサイトURLを登録する
- JWT secret（Dashboard の JWT Settings）を API の `SUPABASE_JWT_SECRET` に入れる。空のときは API がプロジェクトの JWKS で検証する
- `DATABASE_URL` は Go API のみ。anon key 以外をフロントに置かない
- 全テーブルで RLS 有効（マイグレーション済み）。PostgREST からの直接CRUDは許可しない
- 退会時に Auth ユーザーを削除するため、service_role key を API の `SUPABASE_SERVICE_ROLE_KEY` に入れる。フロントエンドには置かない
- 標準SMTPは低レートのため、公開前にカスタムSMTP（Resend 等）を設定する。コーチングを公開するときは、API の `RESEND_API_KEY` と `MAIL_FROM`（保護者の同意のリンクの送信）も同じ Resend のドメインで設定する
- 無料プランは非アクティブで一時停止する可能性がある

## Cloud Run（Go API）

- 初回は [Cloud Run で動かす手順](cloud-run-deploy.md) の `deploy/cloud-run/setup.sh` で作る。以降は Actions の「Deploy API」を手動で流してデプロイする（マージでは自動デプロイしない）
- 環境変数: `DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `SUPABASE_SERVICE_ROLE_KEY`, `CORS_ORIGINS`, `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, `CLOUDINARY_API_SECRET`, `CLOUDINARY_AUTH_TOKEN_KEY`(任意。空なら非公開の動画は API シークレットの署名付き URL で配信する。Cloudinary の有料のトークン認証を有効にしたときだけ鍵を入れると、URL が3時間で切れる), `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`
- `COACHING_ENABLED` はオンラインコーチングを提供するまで設定しない（Web の `NEXT_PUBLIC_COACHING_ENABLED` も同じ）。空のうちは保護者の同意・決済、パーソナル指導、フォーム添削、プッシュ通知、コーチの報酬と送金、運営の送金が 404 になり、コーチの応募と運営の審査だけが残る
- `CORS_ORIGINS` は画面の本番 URL
- ヘルスチェック: `/health`
- 予算（既定は月100円）を超えると API が止まる。再開は手作業

## Stripe（コーチングの支払い・送金）

コーチングを提供するまでは、`COACHING_ENABLED` を空にしたまま準備だけ進められる（空のうちは支払いも Webhook も 404）。
API は、Stripe の鍵だけ入れて Webhook の鍵が無いときや、本番の鍵で https でないときは、起動しない（お金だけ動いて期間が始まらない状態を防ぐため）。

- ダッシュボードで本番に切り替え、本番用の鍵を作る（サンドボックスの鍵・Webhook の鍵は本番では使えない）。秘密鍵は `STRIPE_SECRET_KEY`（`sk_live_...`、または必要な権限だけの制限付きキー `rk_live_...`）
- 支払い方法（カード・Apple Pay・Google Pay・PayPay）は、ダッシュボードの「支払い方法」で有効にしたものが支払い画面に出る。PayPay は審査が要る場合がある
- Connect を有効にする（コーチへの送金。コーチは Express アカウントで、国は日本）
- Webhook の送り先を作る。URL は `https://（API の URL）/webhooks/stripe`
  - 自分のアカウント用: `checkout.session.completed`、`checkout.session.async_payment_succeeded`、`charge.dispute.created`。署名の鍵を `STRIPE_WEBHOOK_SECRET` に入れる
  - 接続アカウント用: `account.updated`。署名の鍵（別の値）を `STRIPE_CONNECT_WEBHOOK_SECRET` に入れる
- `STRIPE_SECRET_KEY` と `STRIPE_WEBHOOK_SECRET` を Cloud Run の環境変数に入れて、「Deploy API」を流す。起動ログの `stripe mode=live` で本番の鍵だと確かめる
- 提供を始めるときに、`COACHING_ENABLED`（API）と `NEXT_PUBLIC_COACHING_ENABLED`（Web）を入れ、Web も「Deploy Web」を流す
- 公開前に、少額で支払い→返金までを通して確かめる（本番の鍵で自分のカードを使い、運営の「パーソナル」タブから返金する）
- 特定商取引法に基づく表記のページ（事業者名・所在地・連絡先・返金の条件など）が公開前に必要。事業者の情報は運営が用意する

## Netlify（Next.js）

- 本番は Actions の「Deploy Web」を手動で流す（`main` をビルドフックでビルド）。`apps/web` をビルドする
- `NEXT_PUBLIC_API_BASE_URL`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `NEXT_PUBLIC_SITE_URL`
- `NEXT_PUBLIC_API_BASE_URL` は Cloud Run の API の URL

## 管理者

`profiles.is_admin = true` を SQL で自分のUUIDに設定する。通報一覧は `GET /v1/admin/reports`。問い合わせ一覧は `GET /v1/admin/inquiries`。非表示は `POST /v1/admin/posts/{id}/hide`。

## コーチング改革の移行・限定公開

フェーズ1〜5とフェーズ6の追加実装を公開する前の手順。ローカル検証の成功だけで本番への適用済みとは扱わない。

### 受付と既存対応の切り替え

| 設定 | 用途 |
|---|---|
| `COACHING_ENABLED=1` | APIの既存契約・会話・Webhook・返金・定期処理を稼働する |
| `NEXT_PUBLIC_COACHING_ENABLED=1` | Webのコーチング画面を表示する（ビルド時に設定） |
| `COACHING_NEW_ACCEPTANCE_ENABLED=1` | 新しい指名申込・添削依頼・チケット購入・追加相談の送信／支払いを受け付ける |

- 新規受付は既定で停止。APIを更新した後に既存の公開設定だけでは受付が再開しないことに注意する。
- 問題時は `COACHING_NEW_ACCEPTANCE_ENABLED` だけを空にしてAPIを再デプロイする。APIとWebの `COACHING_ENABLED` は維持し、購入済みの履行と返金を続ける。
- 受付停止中も、既に申込済みの指名契約の受諾・基本料金の支払い、既存の添削回答・会話・通話／対面予約、返金、送金、停止前に作成した決済画面への遅延通知を受け付ける。追加相談の新しい送信・支払いとチケットの追加購入は止める。下書きの保存と閲覧は続けられる。
- `COACHING_ENABLED` 自体を切るとWebhookと定期処理まで404になるので、取引開始後の受付停止には使わない。

### 移行前の確認

1. DBのバックアップと復元手順を確認し、ステージングへ復元する。旧依頼・旧契約の件数、状態別件数、料金・含有回数・開始終了日時、決済と報酬を保存する。会話本文や保護者の秘密リンクは監査メモへコピーしない。
2. 既存ファイルは変更せず、追加マイグレーション `000093`〜`000099` を順に適用する。`000098` の返金理由と `000099` の対面条件を含める。旧契約は `fulfillment_version=0`、既存のサービス分類なしはそのまま維持し、新しい条件で上書きしない。
3. 復元したDBで旧契約の料金・回数・終了条件が変わっていないこと、旧都度払いとチケットが同じ依頼で二重精算されないことを確認する。旧契約の受諾・保護者決済・通常メッセージ・返金も確認する。
4. 新規受付を停止したままAPI/Webを更新する。Webhookの署名、定期処理の認証と実行履歴、管理者以外が会計照合を読めないことを確認する。
5. コーチ誓約第2版を案内する。再同意前も既存プランが表示され、対面の公開にだけ再同意が必要であることを確認する。

### 招待制の試験運用

1. 利用規約の料金・無料確認・返金・保存期間、プライバシーポリシー、事業者情報を確認し、必要な案内を公開する。本番決済・メール・動画の設定を確認する。
2. 初期の対象コーチはランダムと指名の動画添削に絞る。大会サポート・対面のプランは下書きにする。この受付フラグはサービス別制御や招待認証の代わりにはならない。既存の招待コード制の動作も確認する。
3. 参加者を案内してから新規受付を開ける。Stripeのテスト環境で購入→回答→報酬、未回答→返却、部分利用→残りの返金、通知の重複・遅延、保護者支払い、受付停止後の既存対応を確認する。本番での実決済試験と公開設定の変更は運営が実施する。
4. 日次の会計照合を記録し、不一致や返金の滞留が解消されてから大会サポート、対面の順にプランを公開する。未実施分の返金と双方の実施確認をそれぞれ検証する。

### 日次の照合と復旧

- 管理画面の「コーチングの会計照合」（`/admin/coaching/reconciliation`）または `GET /v1/admin/coaching/reconciliation` を管理者で開く。一つのDBスナップショットで、残高と台帳、支払い済み購入と付与、チケット利用確定と報酬の金額・率を照合する。利用確定後に元の依頼・相談が削除された記録は、残る参照IDがある範囲で照合する。
- 不一致は0件を確認する。送金待ちは月次送金まで通常発生する。返金待ちの件数も異常件数とは分け、日付・前日件数・調査した購入IDなどを運用記録へ残す。件数だけでは同じ返金の滞留を特定できないので、個別管理画面で状態を追う。
- チケット・追加相談の返金待ちは各管理画面から照合・再試行する。大会サポート・対面は契約の返金状態と定期処理の `support_settled` を確認する。処理件数は返金完了件数ではない。
- Stripe側の支払い・返金・送金を同日のアプリ記録と照合する。この画面は外部への照会や、Stripe側だけに存在する支払い／手動返金の検出を行わない。
- 不一致時は新規受付を停止し、通知・定期処理の失敗ログと対象IDを調べる。付与や報酬を手で追加する前に、既存の操作IDで処理が完了しているか確認する。DBを以前の状態へ戻すと外部決済との食い違いが起きるため、バックアップへの復元やマイグレーションの巻き戻しを取引の復旧手順として実行しない。
