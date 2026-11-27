-- 000196 の search_path 固定を外し、関数を呼び出し側の search_path に戻す。
ALTER FUNCTION public.restore_deleted_earning_offsets() RESET search_path;
ALTER FUNCTION public.coaching_access_logs_immutable() RESET search_path;
ALTER FUNCTION public.guard_retired_cloudinary_media() RESET search_path;
ALTER FUNCTION public.link_review_payment_earning() RESET search_path;
ALTER FUNCTION public.snapshot_refund_application() RESET search_path;
ALTER FUNCTION public.reverse_coach_refund_applications(uuid) RESET search_path;
ALTER FUNCTION public.coaching_default_ticket_purchase() RESET search_path;
ALTER FUNCTION public.coaching_issue_referral(uuid,text) RESET search_path;
ALTER FUNCTION public.coaching_issue_referral_code(uuid,text,uuid,uuid) RESET search_path;
ALTER FUNCTION public.coaching_count_referral(uuid,text,uuid,uuid) RESET search_path;
ALTER FUNCTION public.coaching_record_referral(uuid,text,uuid,uuid) RESET search_path;
ALTER FUNCTION public.coaching_cancel_referral() RESET search_path;
ALTER FUNCTION public.coaching_cancel_referral_for_payment_intent(text) RESET search_path;
ALTER FUNCTION public.coaching_reopen_referral_cycle() RESET search_path;
ALTER FUNCTION public.copy_support_record_contract_end_on() RESET search_path;
ALTER FUNCTION public.coaching_grant_referral(text) RESET search_path;
ALTER FUNCTION public.coaching_cancel_referral_count(uuid) RESET search_path;
