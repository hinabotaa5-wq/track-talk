DROP TRIGGER identity_profile_retirement ON public.profiles;
DROP TRIGGER identity_retirement_delete ON public.coach_profiles;
DROP TRIGGER identity_retirement_status ON public.coach_profiles;
DROP TRIGGER identity_coach_insert_delete ON public.coach_profiles;
DROP TRIGGER identity_coach_change ON public.coach_profiles;
DROP TRIGGER identity_plan_insert_delete ON public.coaching_plans;
DROP TRIGGER identity_plan_change ON public.coaching_plans;
DROP SCHEMA identity_boundary CASCADE;
