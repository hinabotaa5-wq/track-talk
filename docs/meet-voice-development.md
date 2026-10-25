# 実況ルーム音声通話：開発・公開前確認

2026-10-25。フェーズ1〜5のコード・自動検証・段階公開の制限を実装。開発用LiveKit Cloudでルーム作成・削除、4つのSDK接続、双方向のOpusパケット転送、参加券失効と再参加を確認済み。**人の声を使った端末間の音声確認、実機4人確認、本番公開は未実施。** 機能は既定で無効。

## 実装済みの動作

- 実況投稿につき音声ルーム1つ、最大4人。聞くだけの参加も定員に含む。通常投稿・削除／非表示投稿には参加できない。
- 公式Go SDK v2.16.0（Go 1.25対応）とブラウザーSDK v2.23.0。マイク送信と音声受信だけを許可する2分の参加券。カメラ・画面共有・録音・文字起こし・一般参加者の管理操作はなし。
- 入室操作まで接続せず、入室時はマイクOFF。マイクON時だけ使用許可を求める。拒否しても受信を継続する。
- 参加者4枠、発言中・ミュート・再接続の表示、通話音量、音声再生許可の案内、退出。動画には独立した音量調整を追加し、端末が反映しない場合は本体／プレーヤーでの操作を案内する。視聴位置の同期はしない。
- ログアウト・ページ移動・画面破棄時に音声・マイクを停止し、退出APIで予約を解放。接続中やマイク許可待ちの取り消しも後始末する。
- 同一アカウントは全ルームを通して予約1件。DBでアカウント・ルームの操作を直列化する。定員はLiveKitの現在の接続者と未接続の予約を合わせて判断し、LiveKit側にも4人上限を設定する。
- 実Cloudでは `max_participants=4` が返っても、検証で直接発行した5人目の参加券による接続が許可された。LiveKit単独の定員拒否には依存しない。APIの直列化・予約数判定に加え、接続サービスでも参加券を署名する直前に実人数を再確認し、4人以上なら参加券を発行しない。実人数の取得に失敗した場合も発行しない。
- 参加ごとに異なる接続UUIDを使い、ユーザーとの対応はDBで管理。古い退出要求・Webhook・参加券が新しい参加を置き換えない。公開ニックネーム以外は送らず、非公開なら「匿名」。
- ルーム未参加時は60秒、全員退出後は20秒で終了。参加券にも同じ定員設定を含め、ルーム再作成時の上限漏れを防ぐ。
- Webhookの署名・本文ハッシュ・最大サイズを検証し、IDで重複排除。通知時刻・接続ID・参加者SID・ルームSIDで遅延と再作成を区別。人数の正本はLiveKitであり、通知だけで予約を解放しない。
- 参加操作はアカウントごとに1分5回。短い未接続予約が切れたら、現在状態を照合し古い参加券を失効してから回復する。
- 既存の通報一覧に音声の参加者・セッションを統合。同じ通話の相手だけを通報できる。音声は保存せず、対象・日時・理由・参加履歴を記録する。
- 管理画面の「音声通話」でマイクOFF・強制退出・対象ルームへの参加停止／解除、全体停止、公開先制限、概算使用量上限を操作。運営からマイクをONにする機能はない。管理操作をDBに記録する。

## API

| API | 認証・役割 |
| --- | --- |
| `GET /v1/posts/{id}/voice` | ログイン必須。`enabled`, `participants`, `capacity`, `full` |
| `POST /v1/posts/{id}/voice/join` | ログイン必須。`url`, `token`, `room`, `expires_in`, `connection_id`。保存／キャッシュ禁止 |
| `POST /v1/posts/{id}/voice/leave` | ログイン必須。JSONで `connection_id`。古いIDの再送は新しい接続を切らない |
| `POST /v1/posts/{id}/voice/heartbeat` | 接続の本人のみ。`connection_id`, 任意の `confirm`。30秒ごとの接続継続確認 |
| `POST /v1/livekit/webhook` | LiveKitの署名付き通知。重複は204 |
| `GET/PUT /v1/admin/voice` | 管理者のみ。状態・使用量・接続・参加停止／設定 |
| `POST /v1/admin/voice/connections/{connectionID}` | 管理者のみ。`action`: `mute`, `remove`, `ban`。任意の `reason` |
| `POST /v1/admin/voice/bans/{userID}/remove` | 管理者のみ。JSONで `post_id` |
| `POST /internal/voice/tick` | `X-Voice-Tick-Secret` を検証。1分ごとの整理 |

主なエラー：401（未ログイン）、403 `voice_banned`、404（対象外）、409 `voice_full` / `voice_already_joined`、410 `voice_ended`、429 `voice_rate_limited`、503 `voice_unavailable` / `voice_paused` / `voice_cleanup_pending`。参加券をURL・ログ・localStorageに保存しない。

## 費用管理と放置接続の整理

- 対応先は **LiveKit Cloud (`*.livekit.cloud`) のみ**。退出は `RemoveParticipant` に `revoke_token_ts` を明示し、未接続・自動更新された古い参加券も失効する。Go 1.25対応SDKにはこの引数がないため、この操作だけ公式JSON APIの小さなアダプターを使う。失効に失敗したら予約を維持して再試行する。参加停止と全体停止の設定は先に保存され、新規参加を拒否する。
- ブラウザーは30秒ごとに継続を通知する。90秒間届かない接続は定期処理で退出。1人で15分経つと継続確認を表示し、1分以内に確認しなければ退出する。画面ロック・バックグラウンドでの継続は保証しない。
- 定期処理が2分以上動いていない場合、新規参加を止める。運営の全体停止は新規参加を止め、既存接続も終了する。処理未完了は管理画面にエラーを返し、同じ操作で再試行できる。
- 全ルーム合計の月間概算（UTC月）をDBで数える。既定上限は4,000参加者分・分。参加券の準備から退出までを数えるため、実際の請求時間とは一致しない。上限後は新規参加を止め、継続確認／定期処理で既存接続も終了する。
- この概算はCloudの無料枠・通信量を保証するものではない。専用Cloudプロジェクトで実使用量を確認し、契約プランの枠を超えないよう余裕を持って上限を設定する。サービス側の上限・障害でもコメントとYouTubeは継続できる。
- アカウント削除などでDB予約がなくなった接続IDも、定期処理で失効させる。
- Webhook処理記録は30日、終了した参加履歴と操作履歴は90日を目安に整理。対応中の通報に関係する参加履歴は保持する。音声・生のWebhook本文・参加券は保存しない。

## 開発用設定から限定公開まで

1. [LiveKit Cloud](https://cloud.livekit.io/) の開発専用プロジェクトでURL・キーを取得し、無視対象の `.env` に設定する。秘密情報をチャットや公開用変数に貼らない。

```dotenv
MEET_VOICE_ENABLED=1
LIVEKIT_URL=wss://YOUR_DEVELOPMENT_PROJECT.livekit.cloud
LIVEKIT_API_KEY=...
LIVEKIT_API_SECRET=...
MEET_VOICE_TICK_SECRET=...
NEXT_PUBLIC_MEET_VOICE_ENABLED=1
```

2. API起動で `000110`・`000111` を適用する。全追加テーブルにRLSを設定済み。Webを再ビルドする。
3. 開発Cloudプロジェクトへ一時ルームを作ってよい状態で、管理APIの接続を確認する（通常のテストでは実接続しない）。

```bash
cd apps/api
LIVEKIT_TEST=1 go test ./internal/livekit -run '^TestCloud(RoomLifecycle|FourParticipants)$' -count=1 -v -timeout 150s
```

4. CloudのWebhook送信先を開発APIの `https://開発APIホスト/v1/livekit/webhook` に設定する。localhostへは直接配信できない。
5. 1分ごとに `POST /internal/voice/tick` を設定する。Cloud Run用の設定スクリプトは `deploy/cloud-run/voice-tick.sh`。これは明示実行時だけ外部設定を変える。ローカルでは同じURLを `X-Voice-Tick-Secret` 付きで呼ぶ。管理画面で最終実行時刻を確認する。
6. 初期設定は「指定した投稿だけ」を許可し、許可一覧は空。管理画面「音声通話」で検証用の実況投稿IDだけを登録する。まず実機確認を行い、音声品質・失敗・使用量を見てから対象を増やす。環境フラグだけで全ルームを公開しない。
7. 公開前に以下を確認する。実施結果をこの資料に追記し、未確認のまま公開済みと扱わない。

## 検証結果と残る確認

### 実Cloud検証（2026-10-25）

- 開発専用 `TrackTalk Development` を作成。データ保存地域は米国、Agent observabilityは無効。開発用キーはGit管理外の `.env` に所有者だけが読める権限（0600）で保存。Web公開用変数には保存していない。
- `TestCloudRoomLifecycle` と `TestCloudFourParticipants` は成功。一時ルームの作成・削除、STANDARD参加者4人の接続、実人数に基づく5人目への参加券発行拒否、2人の送信元から4人へのOpusパケット受信、接続済み参加者の失効と古い参加券の再接続拒否、新しい接続IDでの枠再利用、未接続参加券の失効を確認。
- これは同じPC上のGo SDKによる合成の無音Opusパケットを使う自動確認。実マイク・スピーカー・ブラウザー・別端末の聴感品質を確認したものではない。テストのルームは削除済み。
- 実Cloud単独での4人上限の拒否は確認できなかったため、アプリ側で確実に参加券発行を制限する。参加APIを経由せず参加券を発行する運用はしない。
- CloudへのWebhook設定、定期処理の設定、検証用公開ルーム、本番公開はまだ未実施。
- 参加券発行直前の人数確認を追加後、接続サービス・設定の単体テストと、一時専用PostgreSQL上の通話API統合テスト18件は成功（2026-10-25）。

自動検証を実装・実行：APIは一時専用PostgreSQLと代替LiveKit APIで検証。認可、同時参加、5人目、重複参加、失効、退出後の枠解放、Webhook、運営権限、ミュート、参加停止、全体停止、概算上限、継続確認、自動退出を含む18件。全APIの既存テストも実行する。

Webは入室時ミュート、マイク拒否、満員、再接続、ログアウト、遅い参加応答の解放、マイク許可待ちの中断、通報を検証。独立したブラウザーテストは `npm run test:e2e:voice`。320pxの未ログイン入口・動画音量、PCのログイン済み満員表示・自動接続なしを確認する。CIにも組み込み済み。Webの単体テストは83ファイル・430件成功。LintとTypeScript確認、公開ビルドの最終結果も合わせて確認する。

**未実施（公開条件）：**

- 実マイク・スピーカーを使った2人／4人の音声送受信と、ブラウザー画面経由の満員表示・退出後の再入室。
- Cloud上の自動更新済み参加券の厳密な失効、実Webhookの受信、定期処理の稼働。通常の接続済み／未接続参加券の失効は実Cloudで確認済み。
- iPhone Safari・Android Chrome・PC、イヤホン／スピーカー、YouTube併用、マイク拒否、自動再生許可、通信切替、画面ロック、別アプリ、ログアウト・ページ移動時の端末のマイク停止。
- 運営のミュート・退室・参加停止・全体停止、Cloud使用量と概算の差、サービス上限時にコメント／動画が使えること。

通常のTurbopackビルドは実行環境の一時ポート制限で失敗したため、公開ビルドは公式の `next build --webpack` 方式で検証する。

## 公式資料

- [Room service API](https://docs.livekit.io/reference/other/roomservice-api/)
- [参加券とCloudでの失効](https://docs.livekit.io/frontends/reference/tokens-grants/)
- [Webhook](https://docs.livekit.io/intro/basics/rooms-participants-tracks/webhooks-events/)
- [Cloudの利用上限](https://docs.livekit.io/deploy/admin/quotas-and-limits/)
- [YouTubeの音量操作](https://developers.google.com/youtube/iframe_api_reference)
