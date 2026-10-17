-- パーソナル指導のやりとりの保存期間（2026-10-17 決定）。期間が終わってから1年は読め、その後は文章と動画を消す。
-- 未解決の通報がある契約は、運営が対応を終えるまで消さない。契約・支払い・報酬の記録は残す。
ALTER TABLE coaching_contracts
    ADD COLUMN messages_purged_at timestamptz;
