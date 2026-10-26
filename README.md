# TrackTalk

陸上競技の投稿・大会情報・練習の記録をつなぐコミュニティ。スマートフォンでは **ホーム／大会／メモ／コーチ／ケガ** の5タブ、PCではヘッダーから移動します。マイページはヘッダー右上の人のアイコンにあります。

本番: https://track-talk.netlify.app

## 現在の公開範囲（2026-10-24）

- 投稿・コメント・動画共有・クラブチームと練習相手の募集・商品レビュー。
- 大会の日程と実況スレッド。開催中の大会は通常ページのヘッダーのトロフィーから開けます。
- 本人だけが見られる感覚メモと競技会の記録。
- ケガのグループでの経験共有と、個人を特定しない集計。
- **コーチングは準備中。現在はコーチ募集・応募内容の確認・運営による審査のみ。** ランダムプラン、指名プラン、チケット購入、新しい指導の受付は閉じています。

利用者向けの説明は `/guide`、投稿ルールは `/guidelines`、利用規約は `/terms`、個人情報の取り扱いは `/privacy` にあります。マイページから開けます。

## 主な画面と動作

| 入口 | 内容 |
| --- | --- |
| `/` | 新しい・人気・募集・レビュー、キーワード検索。投稿カードを左右に送る |
| `/upload` | 文章、YouTube・InstagramのURL、自分の動画、クラブチームや練習相手の募集 |
| `/posts/[id]` | 詳細文の下に画像・動画。コメントのカードから会話を開く |
| `/competitions` | 月・地域別の大会、国内主要大会、今後の世界大会 |
| `/memo` | 感覚メモと競技会の記録。感覚メモを書くとビンに玉がたまる |
| `/coaching` | ランダムプラン・指名プラン・コーチ専用画面の縦並び。現在は準備中と募集を案内 |
| `/coach` | 未ログインはログインへ。未応募は応募案内、承認済みも公開までは準備中 |
| `/coach/apply` | 名前・担当種目・記録・成績や指導経験、SNS／ホームページ、顔写真の応募 |
| `/injury` | ケガのグループと復帰データ。グループの投稿は参加者のみ閲覧できる |
| `/me` | 自分の投稿・保存・アカウント設定・通知・利用ガイド・問い合わせ・退会 |
| `/admin` | アプリ管理。管理者のマイページで「自分の投稿」の上に入口を表示 |

閲覧できる範囲と書き込める範囲は異なります。投稿・いいね・保存・コメント・応募などにはログインが必要です。メールとパスワード、またはGoogleアカウントで登録できます。初回はニックネームと種目を設定します。

コーチ応募のSNS／ホームページは **1〜3件必須**。InstagramなどのSNSでも、所属するクラブチームの公式サイトでも使えます。HTTP・HTTPSのURLだけを受け付け、空白と重複は除きます。顔写真がない応募は承認できません。指導年数・競技年数は応募画面にはありません。募集期間中は本人と運営が内容を確認し、公開後のコーチ紹介に使用します。

同じブラウザで最後に開いたメインタブを記憶し、次回トップから起動した際に復元します。詳細への直接リンクは優先します。詳細画面の下部バーも所属タブの色を保ちます。

## フィードの読み込み

- ホームは12件ずつ取得し、画面付近のカードだけ描画します。表示位置と画像比率を保持します。
- 配置の探索は12件のブロック内に限定し、追加読み込みで過去の全カードを再探索しません。
- 画面幅が同じなら高さを保持し、キーボードやページ更新で組み替わりにくくします。画像を初めて読み込んだ際は実際の比率に合わせて配置します。
- 本文等の保持量と取得キャッシュには上限を設け、遠いページは必要時に再取得します。
- 公開ホームのAPIは `pagination=cursor`、`meta.has_more`、`meta.next_cursor` を使用し、毎回の総件数集計と深いOFFSETを避けます。通常のページ番号方式も残しています。
- 人気順は閲覧中のいいね増減で境界が変わるため、固定順位のスナップショットではありません。

設計、制限、比較条件と計測方法は [docs/home-feed-performance.md](docs/home-feed-performance.md) を参照してください。

## 構成

| 場所 | 役割 |
| --- | --- |
| `apps/web` | Next.js 16、React 19、TypeScript、Tailwind CSS 4の画面 |
| `apps/api` | Go 1.25、chi、pgx、sqlcの業務APIと認可 |
| `apps/api/migrations` | PostgreSQLのスキーマ変更。API起動時に適用 |
| `apps/api/queries` | sqlc用SQL。変更後に `make sqlc` で再生成 |
| `deploy` / `.github/workflows` | Cloud Run・Netlifyへの手動公開と定期処理 |
| `docs` | 運用・設計・公開手順 |

認証はSupabase Auth、ファイル保存はCloudinary、公開先はWebがNetlify、APIが東京のCloud Runです。業務データの操作はGo APIに集約し、ブラウザからPostgRESTで直接更新しません。

## ローカル起動

必要なもの: Node.js 22、Go 1.25、Docker。SQLの再生成にはsqlcが必要です。

```bash
cp .env.example .env
# .envに開発用の認証設定等を入力する。秘密情報はコミットしない
make db
make migrate
make api
# 別ターミナル
cd apps/web
npm ci
npm run dev
```

Webは `http://localhost:3000`、APIは `http://localhost:8080/health`。MakefileはローカルDB（既定 `127.0.0.1:55432`）を使用し、`.env` の本番接続先をそのまま利用しません。

外部サービスを使わないDockerプレビューは `make local`、停止は `make local-down`。専用DBとローカル認証を使い、デモログインは `demo@tracktalk.local` / `demo-password` です。プレビューではコーチング機能を有効にできます。本番のGoogleログイン・決済・アップロードの確認とは別です。

ローカル認証は通常 `LOCALAUTH_TLS_CERT_FILE` と `LOCALAUTH_TLS_KEY_FILE` を必須とし、証明書を読み込めない場合は起動しません。Dockerプレビューだけは `LOCALAUTH_ALLOW_INSECURE_HTTP=1` でHTTPを明示的に許可し、ホストの `127.0.0.1:9998` に限定して公開します。`cmd/localauth` を直接起動する場合の待受先は既定で `127.0.0.1:9999` です。このサービスは本番では使用しません。

## 設定と募集のみの運用

環境変数の全項目は `.env.example`、Cloud Runの雛形は `deploy/cloud-run/env.example.yaml` を参照してください。

| 設定 | 用途 |
| --- | --- |
| `DATABASE_URL` | APIのPostgreSQL接続先 |
| `SUPABASE_URL` / `SUPABASE_JWT_SECRET` / `SUPABASE_JWT_AUD` | JWT検証 |
| `SUPABASE_SERVICE_ROLE_KEY` | 退会時の認証アカウント削除 |
| `CLOUDINARY_*` | ファイル保存と署名 |
| `NEXT_PUBLIC_API_BASE_URL` / `NEXT_PUBLIC_SUPABASE_URL` / `NEXT_PUBLIC_SUPABASE_ANON_KEY` / `NEXT_PUBLIC_SITE_URL` | ブラウザ向けの公開設定 |
| `NEXT_PUBLIC_POSTHOG_KEY` / `NEXT_PUBLIC_POSTHOG_HOST` | 任意の匿名の利用状況計測。未設定なら無効。接続・無料枠の確認は [計測の説明](docs/analytics.md) |
| `COACHING_ENABLED` / `NEXT_PUBLIC_COACHING_ENABLED` | 現在は両方 `0`。APIの利用者向け機能を閉じ、画面は準備中にする。応募・審査は継続 |
| `COACHING_NEW_ACCEPTANCE_ENABLED` | 現在は `0`。画面を公開する場合も、新しい申込み・購入の受付は別途判断 |
| `STRIPE_*` | 公開後の決済・返金・送金。募集だけの間は購入画面を提供しない |
| `VAPID_*` | 端末へのプッシュ通知 |
| `SENSE_SUMMARY_ENABLED` / `NEXT_PUBLIC_SENSE_SUMMARY_ENABLED` | 感覚メモのAI振り返りの公開設定 |
| `SENSE_SUMMARY_DAILY_LIMIT` / `SENSE_SUMMARY_MINUTE_LIMIT` | 要約APIの全員合計の試行上限。既定は直近24時間400回・直近1分10回（失敗も含む）。[設定と根拠](docs/sense-summary-limits.md) |
| `GEMINI_API_KEY` | 大会データの抽出・提供中のメモ振り返り |
| `SMTP_*` / `RESEND_API_KEY` / `MAIL_FROM` | 提供中のメール送信 |
| `REMINDER_SECRET` / `STORAGE_CLEANUP_SECRET` | 月1回のお知らせ・保存期限の毎日の整理を Cloud Scheduler から呼ぶときの秘密の値。`deploy/cloud-run/monthly-reminder.sh`・`storage-cleanup.sh` が作って入れる。空なら呼べない(404) |
| `STAFF_MFA_REQUIRED` | `1` で管理者・承認済みコーチの API に二段階認証を求める。全員が `/me/security` で設定してから立てる |
| `NEXT_PUBLIC_TURNSTILE_SITE_KEY` | 登録・ログイン・パスワード再設定の自動操作対策(Cloudflare Turnstile)。先にこれを入れて Web を出し直し、そのあと Supabase の CAPTCHA を有効にする |

本番Webの募集のみ設定は `netlify.toml` の `NEXT_PUBLIC_COACHING_ENABLED = "0"`。APIは「Configure Coaching Availability」ワークフローを `enabled=false` で実行して閉じます。画面のフラグだけでAPIを保護しないでください。再公開時は説明・規約・プライバシーポリシー、決済・返金・未成年者の同意を確認し、両側の設定を合わせます。

秘密鍵、DB接続文字列、service role、Cloudinary secret、Stripe secretはAPI・ホスティング・GitHub Secretsのみに保存します。ブラウザには `NEXT_PUBLIC_*` の公開値だけを渡してください。

## 検証

```bash
make test-api
make test-web
cd apps/web
npm run lint
npx tsc --noEmit
npm run test:e2e
```

API統合テストにはローカルのPostgreSQLが必要です。CIは専用PostgreSQLでAPIを検証し、WebのLint・型・単体テスト、ログインから投稿までの操作、大量投稿の追加読み込みと位置復元を確認します。DBを使わずスキップされたテストだけで本番の動作確認が完了したと判断しないでください。

## 公開と運用

1. 変更をまとめてテストし、mainへ送信します。pushだけでは本番デプロイしません。
2. APIに変更があればActionsの「Deploy API」を実行し、マイグレーションとヘルスを確認します。
3. 募集のみの設定をAPI・Webで合わせます。
4. 「Deploy Web」でNetlifyの公開ビルドを開始します。ワークフロー成功だけでなく、Netlifyの公開状態と対象コミット、実際の画面・APIを確認します。

API更新は同じイメージを予算停止用サービスにも反映します。既存の依頼や返金がある状態で公開範囲を変える場合、利用者への案内と対応経路を先に確認してください。

管理者は `profiles.is_admin = true` の利用者です。マイページの「アプリ管理」から、通報・問い合わせ・商品・コーチ応募の審査などを扱えます。管理APIはサーバー側で権限確認します。

大会の自動取り込みは `.github/workflows/imports.yml`、コーチングの期限処理等は `deploy` 内の運用スクリプトを参照してください。詳しい公開手順は [docs/cloud-run-deploy.md](docs/cloud-run-deploy.md)、確認項目は [docs/launch.md](docs/launch.md) にあります。
