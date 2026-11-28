-- 本体 #1103: 日次の storage cleanup が support_* の削除で permission denied（42501）になり、後段の削除が止まっている。
-- 公開 API の最小権限の実行ロール（track_talk_api、本番で運営者が作成済み）に、次の2つが使う権限だけを足す。
--   DELETE：保存期限を過ぎた記録の削除（本体 #981、queries/support_record_retention.sql）。
--   UPDATE：契約の行が消える時（退会の CASCADE を含む）に契約終了日を書き写すトリガ
--           copy_support_record_contract_end_on（000190）。SECURITY DEFINER ではないので、呼んだロールの権限で動く。
-- SELECT・INSERT は以前から API が使っている権限なので、ここでは触らない。
-- ロールの無い環境（手元・CI）でも失敗しないよう、ロールがあるときだけ付与する。
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'track_talk_api') THEN
        GRANT DELETE, UPDATE ON TABLE
            public.support_contract_statements,
            public.support_coach_settlements,
            public.support_coach_advances,
            public.support_fault_determinations
        TO track_talk_api;
    END IF;
END
$$;
