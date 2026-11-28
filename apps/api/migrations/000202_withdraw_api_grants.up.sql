-- 退会（DELETE /v1/me）が、公開 API の実行ロール（track_talk_api）に
-- coaching_contracts への DELETE 権限が無いため 42501 で失敗し、500 になっていた問題の修正。
-- あわせて、退会のトランザクションが触る書き込みを監査し、不足していた権限をまとめて付与する
-- （000200 と同じ形。ロールが無い環境では何もしない）。
-- 付与する権限は、退会の経路（account.go の deleteAccount／purgeAccount と、
-- その中で呼ばれる報酬記録）が実行する書き込みだけに限る。読み取りは対象にしない。
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'track_talk_api') THEN
        GRANT UPDATE, DELETE ON public.comments TO track_talk_api;
        GRANT UPDATE, DELETE ON public.posts TO track_talk_api;
        GRANT DELETE ON public.practice_ratings TO track_talk_api;
        GRANT DELETE ON public.practice_applications TO track_talk_api;
        GRANT DELETE ON public.reports TO track_talk_api;
        GRANT UPDATE ON public.guardian_consents TO track_talk_api;
    GRANT DELETE ON public.guardian_manage_links TO track_talk_api;
        GRANT UPDATE ON public.document_consents TO track_talk_api;
        GRANT UPDATE ON public.minor_purchase_declarations TO track_talk_api;
        GRANT INSERT ON public.deleted_accounts TO track_talk_api;
    GRANT INSERT, UPDATE ON public.daily_usage_counts TO track_talk_api;
        GRANT UPDATE, DELETE ON public.coach_profiles TO track_talk_api;
        GRANT DELETE ON public.coaching_contracts TO track_talk_api;
        GRANT DELETE ON public.profiles TO track_talk_api;
        GRANT INSERT, UPDATE, DELETE ON public.coach_earnings TO track_talk_api;
        GRANT INSERT ON public.support_coach_advances TO track_talk_api;
        GRANT INSERT ON public.support_coach_settlements TO track_talk_api;
        GRANT INSERT, UPDATE ON public.coach_refund_adjustments TO track_talk_api;
    GRANT INSERT ON public.review_payment_earning_links TO track_talk_api;
    GRANT UPDATE ON public.coach_refund_applications TO track_talk_api;
    GRANT INSERT ON public.coach_refund_application_reversals TO track_talk_api;
    END IF;
END $$;
