# AUTHZ 指摘の確認（2026-10-25）

対象は Semgrep の findings 1040053283 / 1040053282 / 1040053281 / 1040053280 / 1040053279。
行番号は検出コミット c60da79aec3044063065a94481e74adc048a87f0 のもの。

## 予算停止入口（1040053283）

従来は Cloud Run IAM を信頼境界としていた。今回、アプリにも Pub/Sub の Google 署名付き ID トークン検証を追加した。
RS256 署名、有効期限、Google issuer、固定 audience、サービスアカウントの email と email_verified を確認してから通知を処理する。設定・認証が欠ける場合は実行しない。

PUSH_AUDIENCE は Cloud Run のサービス URL、PUSH_SERVICE_ACCOUNT は Pub/Sub push に設定した budget-stop サービスアカウント。setup.sh と deploy-api.yml が両方を設定する。デプロイ時には Cloud Run の Invoker IAM check と未認証アクセス拒否を明示する。

署名鍵は Google の固定 JWKS URL から取得する。リクエストの Host や JWT が指定する任意の URL は使用しない。

認証ヘッダーは Pub/Sub 標準の Authorization を使用する。X-Serverless-Authorization への付け替えは行わない。

参考: https://docs.cloud.google.com/pubsub/docs/authenticate-push-subscriptions

## コーチの依頼操作（残り4件）

以下はルート宣言だけでは分かりにくいが、認証ミドルウェアと処理内部の所有者チェックが既に存在する。今回確認した範囲では認可欠落は再現せず、誤検知としてレビューできる根拠がある。

| finding | ルート | 所有者チェック |
| --- | --- | --- |
| 1040053279 | GET /coach/requests/{id} | GetCoachReviewRequest は id とログイン本人の coach_id を条件に読む |
| 1040053280 | POST /coaching/requests/{id}/response | MarkReviewReturned は coach_id・状態・期限を条件に更新し、成功したトランザクション内だけで返答を作る |
| 1040053281 | POST /coaching/requests/{id}/clarification | MarkReviewClarified は coach_id・状態を条件に更新し、同一トランザクション内だけで補足を保存する |
| 1040053282 | POST /coach/offers/{id}/accept | 承認済みコーチであることを確認し、LockOpenReviewOffer は id と本人の coach_id を条件にロックする |

SQL は apps/api/queries/review_response.sql と review.sql。担当IDを入力JSONから受け取らず、認証されたプロフィールから渡している。

## 検証

- cmd/budgetstop の全テスト: 成功。署名・issuer・audience・送信者・期限・アルゴリズムが不正な場合、トークン欠落、認証関数未設定を含む。
- TestReviewOfferFlow / TestReviewResponseFlow / TestReviewAuthorization: 専用の一時 PostgreSQL で成功（スキップなし）。未ログイン、他コーチ、依頼者本人によるコーチ操作を拒否し、正規担当者の操作が成功することを確認した。
- setup.sh の構文検査と git diff --check: 成功。

ローカルでの修正・検証まで。本番 IAM の実測、デプロイ、Semgrep の再スキャン・ステータス変更は未実施。警告の抑制コメントは追加していない。
