# Connect Webhook の本番準備（2026-10-29）

## 今回の変更と公開範囲

`POST /webhooks/stripe/connect` を追加する。コーチングの公開フラグによらず、`STRIPE_CONNECT_WEBHOOK_SECRET` で署名検証できた `account.updated` だけを処理する。通常の署名キーは受け付けず、Checkout・返金・Transfer の処理には入らない。Stripe API キーまたは Connect 署名キーが未設定なら404。

既存の口座同期を再利用し、既知の `stripe_account_id` に限って Stripe API から最新 Account を取得し、`coach_profiles.payouts_enabled` を保存する。同じ口座の行ロックを取得してから取得・保存するため、重複・古い通知・並行配信による古い値の上書きを防ぐ。Stripe取得・DB失敗は500で再送対象。未知の口座は200で無視する。

この同期は本人確認の詳細や `charges_enabled` をDBへ保存しない。`payouts_enabled` を送金対象判定に使う既存設計を維持する。本人確認要件の詳細と運営者アカウントの charges/payouts の最終確認は Dashboard で行う。

本番への反映が必要。今回の作業ではデプロイ・フラグ変更・Stripe設定変更・実決済を行わない。特商法ページやWebのデプロイとも分ける。

## 通常の決済Webhookの確認結果

2026-10-29、main `3b170c1` では `/webhooks/stripe` は `requireCoaching` 配下にあり、`COACHING_ENABLED` がオフなら署名検証前に404を返す。`checkout.session.completed`、`checkout.session.async_payment_succeeded`、`charge.dispute.created` も同じ。今回の変更でもこの動作は維持する。

本番の `https://track-talk-api-y2wva5o5zq-an.a.run.app` に対し、`/health` は200、通常Webhookへ署名なしの空JSONを送る診断は404だった。Connect専用パスも未デプロイのため404だった。実イベント・正しい署名・秘密値は送受信していない。

この診断だけでは本番フラグの実値やStripeの配信履歴は確定しない。ただし「フラグがオフでも通常の決済通知が処理される」という前提はコード上成立しない。Stripe Dashboard の通常Webhookの配信履歴で、既存の `checkout.session.completed` 等に404がないか、未配信の取引がないかを運営者が確認する。実決済テスト前に、通常Webhookを動かす方針を別途決める必要がある。今回のConnect受信口追加だけでは通常決済の問題は解消しない。機能公開のためにフラグを独断で開けたり、配信済みイベントを無条件に再送したりしない。

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
9. 既知のExpress口座の実際の更新イベントが発生したときに、Stripeの最新 `payouts_enabled` とDBの対象コーチの値が一致することを確認する。テストのために実口座の本人確認・入金設定を不用意に変更しない。口座単位のDB確認が未実施なら、その旨を残す。
10. 必要な既存イベントを運営者が選んで再送し、同じイベントを再送しても支払い・送金を起こさず口座状態が一致することを確認する。署名エラーは400、既知口座の一時取得失敗は500なので、設定や取得権限を直してから再試行する。

Stripe Connect本番送信先にはテストモードのイベントが届く場合もある。既存処理と同じくイベント本文の状態を信用せず、本番キーで既知の口座の現在状態を読み直す。未知のテスト口座は無視する。Dashboardのテスト送信機能が利用できなければ、本番疎通は未実施として残し、サンドボックスの別送信先で先に検証する。

## 本番投入前に確認する差分

- `server.go`：Connect専用パス追加。通常Webhookの公開条件は維持。
- `stripe_connect_webhook.go`：専用署名・イベント限定・本文サイズ制限・既存同期呼び出し。
- `coaching_payment.go`：口座同期で行ロックを取得してからStripe最新状態を取得・保存。
- テスト：フラグオフでの同期、同一イベントの再送、古い本文、排他、署名違い、未知口座、取得失敗と再試行、キー未設定、通常決済イベントの拒否、既存ルートの404を確認。
- 本手順書と公開チェックリストのリンク。

DBスキーマ、料金、Checkout作成、決済・返金・送金ルール、利用規約、Web画面、フラグの変更はない。

## 参照

- [Stripe公式：Connect webhooks](https://docs.stripe.com/connect/webhooks)
- [Stripe公式：Webhookの登録・Snapshotペイロード](https://docs.stripe.com/webhooks)
