# 感覚メモのAIまとめの回数制限

## 使うAIと上限

まとめはAnthropicのClaude APIを使い、既定モデルは `claude-haiku-5-5`（`apps/api/internal/claude/claude.go` の `DefaultModel`）。大会の取り込み用Geminiとは設定も用途も分けています。

- 全員合計で直近24時間400回（`SENSE_SUMMARY_DAILY_LIMIT=400`）。
- 全員合計で直近1分10回（`SENSE_SUMMARY_MINUTE_LIMIT=10`）。
- 本人のまとめ保存は日本時間の1日3回まで（従来どおり）。
- 1回に送るのは300文字以下のメモ6個。返事は考える分を含めて4,096トークンまで、考える量は `low`。返事の形は JSON スキーマで決めている。

有料APIのため、上限は費用と負荷を抑えるために設けています。金額はモデル・入出力量・最新の料金に依存し、回数上限だけでは保証できません。利用前に料金を確認し、Claude Consoleの上限額（Spend limit）も設定します。

モデルは `SENSE_SUMMARY_MODEL` で変えられる。空なら `claude-haiku-5-5`。

## 安全の仕組み

APIを呼ぶ直前にDBで試行枠を予約する。成功分だけでなく、通信失敗・AI側のエラー・不正な応答・保存失敗でも回数を返さない。同時に複数の利用者・APIインスタンスが頼んでも、DBのトランザクションロックで数の確認と予約を直列化する。外部APIの応答待ちのあいだはDBロックを持たない。本文・利用者ID・応答は回数管理表に保存しない。

感覚メモ不足など、APIを呼ばないリクエストは試行枠を使わない。期限切れの試行記録は次の予約時に消す。マイグレーション113で導入前25時間の成功分も初期登録する。導入前の失敗回数は過去に記録していないため復元できない。

空・未設定なら400回/24時間、10回/分を使う。`0`を明示するとその上限を無効にするため、費用を制限する運用では指定しない。上限に達したときはAIを呼ばず、既存の「いまはまとめを作れません」の応答を返す。自動再試行はしない。

## 本番反映

1. [Claude Console](https://platform.claude.com/) で API キーを作り、上限額(Spend limit)を決める。
2. APIキーをSecret Managerに保存し、Cloud Runの `ANTHROPIC_API_KEY` は承認した数値バージョンへの参照として設定する。実行サービスアカウントに対象の秘密の読み取り権限を付ける。モデルを変えないなら `SENSE_SUMMARY_MODEL` は入れない。

```sh
gcloud run services update track-talk-api \
  --project refined-bolt-493709-a8 --region asia-northeast1 \
  --update-secrets 'ANTHROPIC_API_KEY=<秘密名>:<数値バージョン>' \
  --remove-env-vars GEMINI_SUMMARY_MODEL
```

3. APIのコード変更がある場合は「Deploy API」を流す。Webのプライバシーポリシーや案内を変更する場合は「Deploy Web」も流し、利用者向けの説明と実装を合わせる。

大会の県別自動取り込みはGemini（`gemini-3.8-flash`）を使います。公開されている大会ページを対象とし、1回の定期実行は15リクエストまで、県内の複数ページ間も15秒あける設定です。手動実行も同じプロジェクトの利用枠を消費するため、契約ごとの料金・クォータを確認してください。
