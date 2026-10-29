# Connect Webhook の本番準備（2026-10-29）

## 今回の変更と公開範囲

`POST /webhooks/stripe/connect` を追加する。コーチングの公開フラグによらず、`STRIPE_CONNECT_WEBHOOK_SECRET` で署名検証できた `account.updated` だけを処理する。通常の署名キーは受け付けず、Checkout・返金・Transfer の処理には入らない。Stripe API キーまたは Connect 署名キーが未設定なら404。

既存の口座同期を再利用し、既知の `stripe_account_id` に限って Stripe API から最新 Account を取得し、`coach_profiles.payouts_enabled` を保存する。同じ口座の行ロックを取得してから取得・保存するため、重複・古い通知・並行配信による古い値の上書きを防ぐ。Stripe取得・DB失敗は500で再送対象。未知の口座は200で無視する。

この同期は本人確認の詳細や `charges_enabled` をDBへ保存しない。`payouts_enabled` を送金対象判定に使う既存設計を維持する。本人確認要件の詳細と運営者アカウントの charges/payouts の最終確認は Dashboard で行う。

本番への反映が必要。今回の作業ではデプロイ・フラグ変更・Stripe設定変更・実決済を行わない。特商法ページやWebのデプロイとも分ける。

## 通常の決済Webhook：本番の現状と修正後

2026-10-29、main `3b170c1` では `/webhooks/stripe` は `requireCoaching` 配下にあり、`COACHING_ENABLED` がオフなら署名検証前に404を返す。`checkout.session.completed`、`checkout.session.async_payment_succeeded`、`charge.dispute.created` も同じ。今回の変更でもこの動作は維持する。

本番の `https://track-talk-api-y2wva5o5zq-an.a.run.app` に対し、`/health` は200、通常Webhookへ署名なしの空JSONを送る診断は404だった。Connect専用パスも未デプロイのため404だった。実イベント・正しい署名・秘密値は送受信していない。

今回、通常の `/webhooks/stripe` もフラグから切り離す。`COACHING_ENABLED` がオフのときは通常用 `STRIPE_WEBHOOK_SECRET` の署名だけを認め、`checkout.session.completed`、`checkout.session.async_payment_succeeded`、`charge.dispute.created` の3種類だけを既存処理に渡す。通常キーが無い場合は404、署名不正は400、その他のイベントは404。コーチング公開中の既存Connect登録との互換性は維持する。新設Connect専用受信口は、公開状態によらずConnect用署名しか認めない。

これにより、閉鎖前に作ったCheckoutへの遅延通知でも注文を成立させられる。既存の期限切れ・二重払いに対する取消し／返金処理も引き続き実行される。新しい支払い・返金ルールを追加する変更ではない。定期処理は依然フラグの内側にあるため、既存取引の履行・返金運用をWebhookだけで完結したとは扱わない。

### 購入経路の確認

Stripe Checkout作成の呼び出しは次の4系統のみで、入口はすべて `server.go` の `requireCoaching` 配下。フラグオフでアプリから新規Checkoutを作成・取得できる経路は見つからなかった。保護者の同意から決済画面へ進む経路も同じグループ内にある。Webの添削新規申込・依頼詳細・パーソナル指導・保護者・チケット画面もCoachingGate配下。

| 系統 | 購入・決済画面を作る入口 | フラグオフ |
| --- | --- | --- |
| パーソナル指導 | `/v1/coaching/contracts/{id}/checkout`、保護者の契約checkout/consent | 404 |
| 動画添削 | `/v1/coaching/requests`、`/v1/coaching/requests/{id}/checkout` | 404 |
| 追加相談 | `/v1/coaching/consultations/{id}/submit`、本人・保護者のcheckout | 404 |
| 旧チケット | `/v1/coaching/tickets/checkout` | 404。販売停止フラグも維持 |

認証あり・なし双方で購入関係10経路が404となり、偽StripeへCheckout作成が一度も発生しないことをテストした。加えて、閉鎖前の注文へ閉鎖後に通知を送り、パーソナル指導の成立、動画添削カードの仮押さえ反映・PayPayの決済反映、追加相談の入金反映、旧チケットの一度だけの付与を確認する。

これはアプリの入口に関する確認であり、過去に発行済みで有効なStripe Checkout URLを失効させるものではない。そのURLからは停止後でも支払いが完了し得るため、Webhookを独立して動かす必要がある。現在の本番で未失効のCheckoutが0件かどうかは未確認。新規購入経路が閉じていることだけを理由に、過去の取りこぼしも0件とは断定しない。

### Stripe Dashboardの読み取り確認（2026-10-29）

- 本番の通常送信先 `tracktalk-live-payments`（`we_1UVZN6RqDMOeFIE9kjEZkaOv`）はアクティブ、対象イベントは3件。
- 登録URLは `https://track-talk-api-274141774717.asia-northeast1.run.app/webhooks/stripe`。上記のCloud Run URLとはホスト表記が異なるため、デプロイ後の疎通は必ずこの登録URLで確認する。
- 概要の「今週」は配信合計0・失敗0。「イベントの配信」タブ（検索・ステータス指定なし）も「イベント配信は見つかりません」。確認した表示範囲で失敗配信は見つからなかった。全期間・削除済み送信先まで含む保証ではない。
- 本番の「決済用リンク」は新規作成の案内画面で、既存リンクは表示されなかった。Checkout画面も利用開始の案内で、未失効セッションの全件確認はできていない。
- Dashboardは閲覧のみ。送信先作成・設定変更・再送・署名の表示は行わない。

#284は全CI成功後にマージ済みで、本番APIへ反映済み。本番リビジョンは `track-talk-api-00096-mbp`。本番Connect送信先 `tracktalk-live-connect` は稼働中で、送信元はConnected accounts、形式はSnapshot、対象は `account.updated` のみ、URLは `https://track-talk-api-274141774717.asia-northeast1.run.app/webhooks/stripe/connect`。署名付き診断で通常決済・Connectの各受信口の200は確認済みだが、Stripe自身からの実イベント配信と既知コーチ口座のDB同期の確認を終えた意味ではない。PayPay承認後の公開日に手順書Cと手順書Bを実施し、Cでは手順9の配信記録・受信ログの両方、Bでは実注文への反映を確認する。署名なしのHTTP応答や自己署名の診断だけでは、これらの実配信・実決済テストの成功扱いにしない。

## 運営者の設定手順

本番の専用受信口がデプロイされるまでは疎通しない。下記は作業手順であり、設定済みを意味しない。

1. Stripe Dashboard を本番に切り替え、Workbench → Webhooks → イベント送信先の作成へ進む。既存の接続アカウント用送信先があれば重複作成を避ける。通常の決済Webhookは変更しない。
2. イベント送信元は **Connected accounts（接続済みアカウント）** を選ぶ。ペイロードは既存コードに合わせ **Snapshot**、イベントは **`account.updated` のみ** を選択する。APIバージョンは現行連携の設定に合わせる。
3. 送信先を Webhook endpoint とし、URL を次に設定する。
   `https://track-talk-api-y2wva5o5zq-an.a.run.app/webhooks/stripe/connect`
4. 控える情報は送信先ID、URL、本番/接続アカウントの区分、イベント、APIバージョン、署名シークレット。署名シークレットの実値だけはチャット・PR・メモ・スクリーンショットに載せず、Google Cloud Console の Secret Manager の登録欄へ直接渡す。CLI用や通常Webhook用の署名とは別の値。
5. GCPプロジェクト `refined-bolt-493709-a8` の Secret Manager に **`tracktalk-stripe-connect-webhook-secret`** を作成して保存する。2026-10-29の名前一覧にはこのシークレットは存在しなかった。既に作成されていれば新バージョンに保存する。
6. Cloud Run の実行サービスアカウントに、このシークレットに対する Secret Accessor 権限を付ける。Cloud Run `track-talk-api`（`asia-northeast1`）の環境変数 **`STRIPE_CONNECT_WEBHOOK_SECRET`** を、上記シークレットの承認した数値バージョンへの参照として追加する。値の直接入力・既存環境変数の一括置換はしない。
7. レビュー済みのAPI変更と、この参照設定を本番へ反映する。対象差分を確認してから実施し、コーチング系フラグはすべて閉じたままにする。Secret Managerへの保存だけでは稼働中のリビジョンには反映されない。
8. Stripe の送信先画面でテスト送信が利用できる場合は `account.updated` を送り、配信結果が2xxであることを確認する。未知のテスト口座への200は署名・経路の確認であり、既知コーチのDB同期成功とは区別する。
9. 手順書Cでは、既知のExpress口座の実際の `account.updated` について、**Stripe側の配信記録がHTTP 200で、かつ同じイベントの配信に対応するCloud Run側の受信ログが同時刻にHTTP 200であることの両方**をそろえて、Connect同期の確認済みとする。Stripeの最新 `payouts_enabled` とDBの対象コーチの値の一致も確認するが、DBの値だけではWebhookで同期した証拠にならない。コーチの画面操作でもStripeを読み直し、DBをtrueに更新する処理があるため。テストのために実口座の本人確認・入金設定を不用意に変更しない。配信記録・受信ログ・DBのいずれかを確認していなければ、その未確認範囲を残す。
10. 必要な既存イベントを運営者が選んで再送し、同じイベントを再送しても支払い・送金を起こさず口座状態が一致することを確認する。署名エラーは400、既知口座の一時取得失敗は500なので、設定や取得権限を直してから再試行する。

Stripe Connect本番送信先にはテストモードのイベントが届く場合もある。既存処理と同じくイベント本文の状態を信用せず、本番キーで既知の口座の現在状態を読み直す。未知のテスト口座は無視する。Dashboardのテスト送信機能が利用できなければ、本番疎通は未実施として残し、サンドボックスの別送信先で先に検証する。

## 本番投入前に確認する差分

- `server.go`：Connect専用パス追加と、通常決済Webhookの公開フラグからの分離。購入APIは閉じたまま。
- `stripe_connect_webhook.go`：専用署名・イベント限定・本文サイズ制限・既存同期呼び出し。
- `coaching_payment.go`：閉鎖中の通常Webhookを通常署名と決済イベント3種に限定。口座同期は行ロック取得後にStripe最新状態を取得・保存。
- テスト：フラグオフでの同期、同一イベントの再送、古い本文、排他、署名違い、未知口座、取得失敗と再試行、キー未設定、Connect側の通常決済イベント拒否、通常側の決済成立、購入APIの404を確認。
- 本手順書と公開チェックリストのリンク。

DBスキーマ、料金、Checkout作成、決済・返金・送金ルール、利用規約、Web画面、フラグの変更はない。

## 参照

- [Stripe公式：Connect webhooks](https://docs.stripe.com/connect/webhooks)
- [Stripe公式：Webhookの登録・Snapshotペイロード](https://docs.stripe.com/webhooks)
