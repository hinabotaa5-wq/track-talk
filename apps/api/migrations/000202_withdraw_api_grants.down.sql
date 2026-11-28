-- 000202 の取り消し。up で足した権限だけを外す。
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'track_talk_api') THEN
        REVOKE UPDATE, DELETE ON public.comments FROM track_talk_api;
        REVOKE UPDATE, DELETE ON public.posts FROM track_talk_api;
        REVOKE DELETE ON public.practice_ratings FROM track_talk_api;
        REVOKE DELETE ON public.practice_applications FROM track_talk_api;
        REVOKE DELETE ON public.reports FROM track_talk_api;
        REVOKE UPDATE ON public.guardian_consents FROM track_talk_api;
    REVOKE DELETE ON public.guardian_manage_links FROM track_talk_api;
        REVOKE UPDATE ON public.document_consents FROM track_talk_api;
        REVOKE UPDATE ON public.minor_purchase_declarations FROM track_talk_api;
        REVOKE INSERT ON public.deleted_accounts FROM track_talk_api;
    REVOKE INSERT, UPDATE ON public.daily_usage_counts FROM track_talk_api;
        REVOKE UPDATE, DELETE ON public.coach_profiles FROM track_talk_api;
        REVOKE DELETE ON public.coaching_contracts FROM track_talk_api;
        REVOKE DELETE ON public.profiles FROM track_talk_api;
        REVOKE INSERT, UPDATE, DELETE ON public.coach_earnings FROM track_talk_api;
        REVOKE INSERT ON public.support_coach_advances FROM track_talk_api;
        REVOKE INSERT ON public.support_coach_settlements FROM track_talk_api;
        REVOKE INSERT, UPDATE ON public.coach_refund_adjustments FROM track_talk_api;
    REVOKE INSERT ON public.review_payment_earning_links FROM track_talk_api;
    REVOKE UPDATE ON public.coach_refund_applications FROM track_talk_api;
    REVOKE INSERT ON public.coach_refund_application_reversals FROM track_talk_api;
    END IF;
END $$;
