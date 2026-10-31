# Stripe Webhook：決済とConnect口座同期

## 受信口と公開設定

受信口は `apps/api/internal/httpx/server.go` に定義し、コーチングの公開フラグから独立しています。新規の購入APIは `requireCoaching` の内側です。Webhookの受信と、新規の受付・購入の公開は別に確認してください。

| 受信口 | 署名 | 公開フラグがオフのときに扱うイベント |
| --- | --- | --- |
| `POST /webhooks/stripe` | 通常用 `STRIPE_WEBHOOK_SECRET` | `checkout.session.completed`、`checkout.session.async_payment_succeeded`、`charge.dispute.created`、`charge.refunded`、`refund.created`、`transfer.created`、`transfer.reversed`、`payout.created` の8種類 |
| `POST /webhooks/stripe/connect` | 専用 `STRIPE_CONNECT_WEBHOOK_SECRET` のみ | `account.updated` のみ |

Stripe APIキーまたは対象受信口の署名キーがなければ404、署名不正は400です。Connect専用パスに通常用の署名は使えません。Connect専用パスは対象外イベントを400で拒否し、決済・返金・Transferを処理しません。通常パスは公開フラグがオフの間、対象外イベントを404で拒否します。公開中の通常パスでは、既存連携との互換性のためConnect用の署名も認め、`account.updated` も扱います。

閉鎖前に作成済みのStripe Checkout URLからは、受付停止後も支払いが完了し得ます。通常Webhookは、その決済完了と既存の期限切れ・二重払いに対する取消し／返金を処理します。返金・送金の5イベントは、DBに記録のない動きを検知して `OPS_ALERT_EMAIL` に通知するためにも使います。設定は [Cloud Runの手順](cloud-run-deploy.md) を参照してください。

定期処理 `/internal/coaching/tick` は公開フラグの内側なので、Webhookだけで既存取引の履行・確定・返金を完結できるとは扱いません。作成済みCheckoutの件数や本番の配信確認結果は、運用記録で管理します。

## Connect口座の同期

Connect受信口は、既知の `stripe_account_id` に限りStripe APIから最新Accountを取得し、`coach_profiles.payouts_enabled` を保存します。同じ口座の行ロックを取ってから取得・保存するため、重複・古い通知・並行配信による古い値の上書きを防ぎます。イベント本文の状態は同期の正本にしません。

- 未知の口座は200で無視します。
- Stripe取得・DB処理の失敗は500を返し、再送の対象にします。
- 本人確認の詳細や `charges_enabled` はDBへ保存しません。本人確認要件と運営者アカウントのcharges/payoutsの最終確認はDashboardで行います。
- テストモードの口座更新が本番送信先へ届いた場合も、本番キーで既知口座の現在状態を読み直します。未知のテスト口座は無視します。

## 設定手順

以下は送信先を作成・変更する場合の手順で、設定済み・未設定を示す記録ではありません。秘密値を表示せず、既存の送信先・秘密名・参照バージョンを先に確認し、重複を避けます。

1. Stripe Dashboardを本番に切り替え、Workbench → Webhooksで送信先を確認します。通常決済用とConnect用は別の送信先にします。
2. Connect用は送信元を **Connected accounts（接続済みアカウント）**、形式を **Snapshot**、イベントを **`account.updated` のみ** にします。APIバージョンは現行連携と合わせます。
3. URLを本番APIの `/webhooks/stripe/connect` にします。通常決済用は同じAPIの `/webhooks/stripe` とし、上表の8イベントを登録します。登録済みのCloud Run URLを使い、ホスト名の表記が異なる別名へ独断で変更しません。疎通確認にはStripeに登録されたURLを使います。
4. Connectの署名シークレットは通常用・CLI用と別の値です。表示・コピーが必要な作業は運営者が行い、Secret Managerの登録欄へ直接渡します。チャット・PR・文書・画面記録に載せません。
5. Secret Managerの秘密名は `tracktalk-stripe-connect-webhook-secret` を使います。既にある場合は重複作成せず、変更が必要なときだけ新バージョンを保存します。
6. Cloud Runの実行サービスアカウントに対象の秘密だけのSecret Accessor権限を付けます。`STRIPE_CONNECT_WEBHOOK_SECRET` は承認した数値バージョンへの参照とし、値の直接入力・環境変数の一括置換はしません。
7. 対象差分を確認してAPIと参照設定を反映します。秘密の保存だけでは稼働リビジョンへ反映されません。購入を閉じたまま準備するときはコーチング公開・新規受付の設定を維持します。
8. Dashboardのテスト送信が使える場合は `account.updated` を送って2xxを確認します。未知のテスト口座への200は署名・経路の確認であり、既知口座のDB同期成功とは分けます。機能が使えなければサンドボックスの別送信先で先に検証し、本番の未確認範囲を運用記録へ残します。

## 公開日の確認（手順書C → B）

[ランブックの手順書C](runbooks/stripe-live-20261029.html#rb-c) は、PayPay承認後、API・Webを公開し、新規受付を止めた状態で、手順書Bの直前に行います。口座登録APIを公開フラグの外に出しません。開放順は [公開チェックリスト](launch.md) に従います。

既知のExpress口座の実際の `account.updated` について、次の3点をそろえます。

1. Stripe側の同じイベントの配信記録がHTTP 200。
2. 同時刻のCloud Run側の対応する受信ログがHTTP 200。
3. Stripeの最新 `payouts_enabled` とDBの対象コーチの値が一致。

DBの値だけではWebhookの同期を証明できません。コーチの画面操作にもStripeを読み直してDBを更新する処理があるためです。署名なしの応答や自己署名の診断だけを、Stripeからの実配信の成功と扱いません。テストのために実口座の本人確認・入金設定を不用意に変更しません。

必要な既存イベントを運営者が再送する場合は、同じイベントでも支払い・送金が起きず、口座状態が一致することを確認します。署名エラーは400、既知口座の一時取得失敗は500なので、原因を直してから再試行します。

C成功後に新規受付だけを開け、[手順書B](runbooks/stripe-live-20261029.html#rb-b) で実注文への反映を確認します。確認できなかった配信・ログ・DBの範囲は運用記録に残します。

## 購入経路と関連テスト

フラグがオフのとき、パーソナル指導の契約checkout・保護者checkout/consent、動画添削の申込み・checkout、追加相談のsubmit・本人/保護者checkout、旧チケットcheckoutは404です。旧チケットの新規購入には別途販売フラグも必要で、新規販売は終了しています。

主な確認先は `stripe_connect_webhook.go`、`coaching_payment.go`、`server.go` と対応する既存テストです。フラグオフでの同期、再送・古い本文・排他、署名違い、未知口座、取得失敗と再試行、キー未設定、Connect側の決済イベント拒否、通常側の決済成立、購入APIの404を確認します。

## 参照

- [Stripe公式：Connect webhooks](https://docs.stripe.com/connect/webhooks)
- [Stripe公式：Webhookの登録・Snapshotペイロード](https://docs.stripe.com/webhooks)
