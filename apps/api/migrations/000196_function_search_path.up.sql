-- Security Advisor（splinter 0011 function_search_path_mutable）の警告17件を解消する（#1079）。
-- 本体は書き換えず、SET 句で search_path を public に固定するだけ。本体は非修飾のテーブル参照を含むので '' にはしない。
ALTER FUNCTION public.restore_deleted_earning_offsets() SET search_path = 'public';
ALTER FUNCTION public.coaching_access_logs_immutable() SET search_path = 'public';
ALTER FUNCTION public.guard_retired_cloudinary_media() SET search_path = 'public';
ALTER FUNCTION public.link_review_payment_earning() SET search_path = 'public';
ALTER FUNCTION public.snapshot_refund_application() SET search_path = 'public';
ALTER FUNCTION public.reverse_coach_refund_applications(uuid) SET search_path = 'public';
ALTER FUNCTION public.coaching_default_ticket_purchase() SET search_path = 'public';
ALTER FUNCTION public.coaching_issue_referral(uuid,text) SET search_path = 'public';
ALTER FUNCTION public.coaching_issue_referral_code(uuid,text,uuid,uuid) SET search_path = 'public';
ALTER FUNCTION public.coaching_count_referral(uuid,text,uuid,uuid) SET search_path = 'public';
ALTER FUNCTION public.coaching_record_referral(uuid,text,uuid,uuid) SET search_path = 'public';
ALTER FUNCTION public.coaching_cancel_referral() SET search_path = 'public';
ALTER FUNCTION public.coaching_cancel_referral_for_payment_intent(text) SET search_path = 'public';
ALTER FUNCTION public.coaching_reopen_referral_cycle() SET search_path = 'public';
ALTER FUNCTION public.copy_support_record_contract_end_on() SET search_path = 'public';
ALTER FUNCTION public.coaching_grant_referral(text) SET search_path = 'public';
ALTER FUNCTION public.coaching_cancel_referral_count(uuid) SET search_path = 'public';
