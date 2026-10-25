# AUTHZ 指摘の確認（2026-10-26 更新）

## 結論の訂正

前の記録は所有者チェックだけを検証し、残りの指摘を誤検知と判断していた。
Semgrep の説明を直接確認すると、問題は「承認を取り消されたコーチが、以前の割り当てを使い続けられる」ことだった。
所有者であることと現在も承認済みであることは別の条件。停止後のアクセス・操作をローカルDBで再現したため、誤検知の判断を撤回する。

検出コミットは c60da79aec3044063065a94481e74adc048a87f0。指摘IDと行番号の対応も訂正した。

| finding | 検出箇所 | 対象 |
| --- | --- | --- |
| 1040053283 | cmd/budgetstop/main.go:65 | 予算停止通知の受信認証 |
| 1040053278 | internal/httpx/server.go:331 | GET /coach/requests |
| 1040053279 | internal/httpx/server.go:332 | GET /coach/requests/{id} |
| 1040053280 | internal/httpx/server.go:333 | POST /coaching/requests/{id}/response |
| 1040053281 | internal/httpx/server.go:334 | POST /coaching/requests/{id}/clarification |
| 1040053282 | internal/httpx/server.go:336 | POST /coach/offers/{id}/decline |

## 予算停止入口（1040053283）

Google 署名付き Pub/Sub ID トークンを検証する既存の修正は main と本番に反映済み。
RS256署名、有効期限、Google issuer、固定 audience、サービスアカウントの email と email_verified を検証し、設定や認証が欠ける場合は通知を処理しない。
署名鍵は固定の Google JWKS URL から取得し、リクエストの Host やトークン内のURLは使用しない。

2026-10-26 に Google Cloud コンソールで確認した本番の状態:

- track-talk-budget-stop は「認証が必要」、IAM が有効。
- リビジョン track-talk-budget-stop-00048-bbx が100%のトラフィックを処理。
- イメージのタグは 119670bcb20bfbf08e8caf94d6103ee886bcebeb。
- 稼働リビジョンのイメージダイジェストは sha256:e8766fc763f67a15c258be973cad952652637bce412707ce7c53fcd4c7aaf740。
- PUSH_AUDIENCE は https://track-talk-budget-stop-y2wva5o5zq-an.a.run.app。
- PUSH_SERVICE_ACCOUNT は budget-stop@refined-bolt-493709-a8.iam.gserviceaccount.com。
- Pub/Sub の budget-stop サブスクリプションは push 認証が有効。送信先とサービスアカウントは上記の PUSH_AUDIENCE・PUSH_SERVICE_ACCOUNT と一致し、明示的な audience の上書きはない。
- このイメージタグのソースを GitHub から取得し、上記の認証処理を確認した。
- PUSH_AUDIENCE の入口へ認証なしで空の通知をPOSTすると403。停止処理を起こさない空のデータのみを使用した。

本番のIAM設定やデプロイは変更していない。

## コーチの依頼操作（1040053278〜1040053282）

既存の RequireAuth と coach_id による所有者チェックは正しく存在するが、承認状態の確認が欠けていた。
承認済みコーチに打診・依頼を割り当ててから status='suspended', on_duty=false に変更すると、同じトークンで以下を再現できた。

- 打診中の一覧・添削中の一覧・依頼詳細は200で閲覧できる。
- 返答は200で受理され、依頼が in_review から returned に変わる。
- 補足は200で受理され、依頼が clarifying から returned に変わる。
- 辞退は204で受理され、依頼が offered から queued に変わる。

修正では requireApprovedReviewCoach に本人認証と現在の承認状態の確認をまとめ、上記5経路と対応する引受経路で使用する。
DB上の状態が approved 以外の場合は403を返し、依頼データの閲覧・状態変更の前に処理を終了する。
既存の所有者・期限・状態の条件は維持する。

本番の track-talk-api は同じ 119670bcb20bfbf08e8caf94d6103ee886bcebeb のイメージを使用し、COACHING_ENABLED=0。
現在は機能全体が閉じているが、機能を開ける前にこの修正の反映が必要。

## 検証とスキャナ

- TestReviewRevokedCoachAuthorization は、修正前に上記6ケースすべてで失敗し、停止後の閲覧・操作を再現した。
- 修正後は停止済みコーチを403で拒否し、依頼状態が変わらないことを確認。
- TestReviewOfferAuthorization は未認証、不正トークン、他コーチ、依頼者の操作を拒否し、打診が変わらず、所有者の正規操作が成功することを確認。
- cmd/budgetstop と internal/httpx の全テストは成功。失敗・スキップなし。
- gofmt と git diff --check は成功。
- 最新 main から切り出した修正だけのチェックアウトでも go test ./... が成功。実テストの成功イベント481件（サブテストを含む）、外部の実サービス接続が必要な2件は所定のスキップ。
- Semgrep の main に対する通常の全体スキャンは2026-10-26 04:34 JST開始、4分10秒で完了し、Code・Supply Chainとも0件。
- AI-powered detection は利用枠切れで無効。通常スキャンと今回のAI指摘の再検査は同じものではない。
- 予算停止の警告には本番検証の根拠を保存済み。UIの Fixed は再スキャン時の自動判定であり、手動の Ignore 理由には修正済みがないため、誤検知として代用しない。
- 警告を誤検知として無視する操作や、ファイル・ルールの除外は行っていない。

コーチ承認の追加修正はレビュー用PRとして用意し、mainへの取り込みと本番反映前の状態。
警告は実際の修正反映と利用可能な再検査結果に基づいて扱い、未反映の5件を解決済みにはしない。

参考: [Pub/Sub OIDC audience の既定値（Google API リファレンス）](https://developers.google.com/resources/api-libraries/documentation/pubsub/v1/csharp/latest/classGoogle_1_1Apis_1_1Pubsub_1_1v1_1_1Data_1_1OidcToken.html)。未指定の場合は push エンドポイントURLを使用する。
