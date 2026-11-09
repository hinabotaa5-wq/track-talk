-- 動画ファイルの保持を最小ラインにする（運営者決定 2026-11-10、本体 #506）。
-- 動画添削はスレッドの終わりから7日、サポートプランは契約の終わりから30日で、動画ファイル（元ファイルと派生）を消す。
-- 文章・メニュー・取引の記録の保持は変えない。行は残し、動画を消した日時だけを書く（動画の ID は流用を防ぐために残す）。
ALTER TABLE review_requests
    ADD COLUMN videos_purged_at timestamptz,
    -- 保全ホールド。通報・紛争（チャージバックなど）の対応中に運営が立て、対応を終えたら外す。立っている間は動画を消さない。
    ADD COLUMN retention_hold_at timestamptz;

ALTER TABLE coaching_contracts
    ADD COLUMN videos_purged_at timestamptz,
    -- 保全ホールド。立っている間は、終わった後もやりとり欄を閉じず、動画も消さない。
    ADD COLUMN retention_hold_at timestamptz,
    -- 終わる3日前の「終わると見られなくなるので、必要ならダウンロードを」の知らせを送った日時。
    ADD COLUMN close_notice_at timestamptz;

ALTER TABLE coaching_consultations
    ADD COLUMN videos_purged_at timestamptz;
