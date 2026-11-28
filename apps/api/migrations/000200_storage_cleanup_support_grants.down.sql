-- 000200 で足した権限を外す。
-- support_fault_determinations の UPDATE は、帰責判定への異議・見直しで API が以前から使っている権限
-- （queries/support_settlement.sql）なので、戻しても外さない。
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'track_talk_api') THEN
        REVOKE DELETE ON TABLE
            public.support_contract_statements,
            public.support_coach_settlements,
            public.support_coach_advances,
            public.support_fault_determinations
        FROM track_talk_api;
        REVOKE UPDATE ON TABLE
            public.support_contract_statements,
            public.support_coach_settlements,
            public.support_coach_advances
        FROM track_talk_api;
    END IF;
END
$$;
