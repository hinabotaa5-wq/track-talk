# 保存期限による日次の整理

`/internal/storage/cleanup` を Cloud Scheduler から日本時間の毎日4時に呼ぶ。
`STORAGE_CLEANUP_SECRET` と同じ値の `X-Storage-Cleanup-Secret` ヘッダーが必要。
コーチングの提供・新規受付の設定には依存しない。

## 対象

- 通知：届いてから既読90日、未読180日。
- 大会情報：4月始まりの現シーズンより古い `competitions`。国内主要大会と世界大会の開催情報は、終了日が現シーズン開始より前のもの。
- 指導の会話・追加相談：既存の終了から1年の条件と、未解決通報・進行中相談・返金等の保護条件をそのまま使う。契約・決済・報酬の記録は残す。
- 未使用の画像・動画：Cloudinary のアプリ専用フォルダ内で作成から7日を過ぎ、使用先がないもの。削除済み投稿の添付ファイルの取り残しも含む。1回に最大10ファイルを削除し、失敗したものは次回に再試行する。

## 残すもの

普通の投稿・コメント・その添付ファイル、競技記録、感覚メモと要約、ケガの投稿・経過、商品レビュー、フォーム添削の依頼・回答は年数で消さない。
非表示の投稿、コーチの写真、商品画像、指導・相談で使用中のファイルも保護する。

## 競合への対策

マイグレーション100で、ファイルの使用先に書き込みトリガーを付ける。
回収側と書き込み側はファイルごとの同じトランザクションロックを取り、回収側はロック後に使用状況を再確認する。
回収対象を `cloudinary_retirements` に記録してから外部ファイルを消す。
古い投稿画面からそのファイルを再利用すると、選び直しを案内する。
外部削除に失敗しても記録を残し、再利用を防いだまま翌日に削除を再試行する。

## 設定と確認

1. API を新しい版に更新する（起動時にマイグレーション100が適用される）。
2. Cloud Shell で `PROJECT_ID=refined-bolt-493709-a8 bash deploy/cloud-run/storage-cleanup.sh` を実行する。秘密の値は表示しない。
3. Scheduler のジョブ `track-talk-storage-cleanup` を手動実行して、API の `storage cleanup completed` と削除件数を確認する。
4. 停止するときは `gcloud scheduler jobs pause track-talk-storage-cleanup --location asia-northeast1 --project refined-bolt-493709-a8`。

未使用ファイルの列挙・削除には Cloudinary Admin API の上限が適用される。失敗時は `storage cleanup failed` を記録し、Scheduler の失敗として扱う。
