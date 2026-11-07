-- Publication approvals are bound to epochs maintained by the database owner.
-- The application runtime has no DDL privileges or write access to this schema.
CREATE SCHEMA identity_boundary;
REVOKE ALL ON SCHEMA identity_boundary FROM PUBLIC;

CREATE TABLE identity_boundary.coach_epochs (
    id uuid PRIMARY KEY,
    epoch uuid NOT NULL DEFAULT gen_random_uuid()
);
CREATE TABLE identity_boundary.plan_epochs (
    id uuid PRIMARY KEY,
    epoch uuid NOT NULL DEFAULT gen_random_uuid()
);

CREATE FUNCTION identity_boundary.coach_epoch() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM identity_boundary.coach_epochs WHERE id = OLD.profile_id;
        RETURN OLD;
    END IF;
    INSERT INTO identity_boundary.coach_epochs (id)
    VALUES (NEW.profile_id)
    ON CONFLICT (id) DO UPDATE SET epoch = gen_random_uuid();
    RETURN NEW;
END;
$$;

CREATE FUNCTION identity_boundary.plan_epoch() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        DELETE FROM identity_boundary.plan_epochs WHERE id = OLD.id;
        RETURN OLD;
    END IF;
    INSERT INTO identity_boundary.plan_epochs (id)
    VALUES (NEW.id)
    ON CONFLICT (id) DO UPDATE SET epoch = gen_random_uuid();
    RETURN NEW;
END;
$$;

CREATE TRIGGER identity_coach_insert_delete
AFTER INSERT OR DELETE ON public.coach_profiles
FOR EACH ROW EXECUTE FUNCTION identity_boundary.coach_epoch();
CREATE TRIGGER identity_coach_change
AFTER UPDATE ON public.coach_profiles FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status
   OR OLD.pledge_version IS DISTINCT FROM NEW.pledge_version
   OR OLD.service_types IS DISTINCT FROM NEW.service_types)
EXECUTE FUNCTION identity_boundary.coach_epoch();

CREATE TRIGGER identity_plan_insert_delete
AFTER INSERT OR DELETE ON public.coaching_plans
FOR EACH ROW EXECUTE FUNCTION identity_boundary.plan_epoch();
CREATE TRIGGER identity_plan_change
AFTER UPDATE ON public.coaching_plans FOR EACH ROW
WHEN (OLD.coach_id IS DISTINCT FROM NEW.coach_id
   OR OLD.published IS DISTINCT FROM NEW.published
   OR OLD.service_type IS DISTINCT FROM NEW.service_type
   OR OLD.competition_date IS DISTINCT FROM NEW.competition_date
   OR OLD.support_start_date IS DISTINCT FROM NEW.support_start_date)
EXECUTE FUNCTION identity_boundary.plan_epoch();

REVOKE EXECUTE ON FUNCTION identity_boundary.coach_epoch(), identity_boundary.plan_epoch() FROM PUBLIC;
INSERT INTO identity_boundary.coach_epochs (id) SELECT profile_id FROM public.coach_profiles;
INSERT INTO identity_boundary.plan_epochs (id) SELECT id FROM public.coaching_plans;

CREATE VIEW identity_boundary.published_plans AS
SELECT pl.id, pl.coach_id, pl.title, pe.epoch AS plan_epoch, ce.epoch AS coach_epoch
FROM public.coaching_plans pl
JOIN public.coach_profiles c ON c.profile_id = pl.coach_id
JOIN identity_boundary.plan_epochs pe ON pe.id = pl.id
JOIN identity_boundary.coach_epochs ce ON ce.id = c.profile_id
WHERE pl.published
  AND c.status = 'approved'
  AND c.pledge_version >= 1
  AND (pl.service_type IS NULL OR (pl.service_type IN ('competition_support','in_person') AND pl.service_type = ANY(c.service_types)))
  AND (pl.service_type IS DISTINCT FROM 'competition_support' OR
       (pl.competition_date >= (now() AT TIME ZONE 'Asia/Tokyo')::date AND pl.support_start_date IS NOT NULL));

CREATE VIEW identity_boundary.approved_coaches AS
SELECT profile_id FROM public.coach_profiles WHERE status = 'approved';

CREATE FUNCTION identity_boundary.last_transaction(coach uuid) RETURNS timestamptz
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT GREATEST(
        (SELECT max(created_at) FROM public.coach_earnings WHERE coach_id = coach),
        (SELECT max(paid_at) FROM public.coach_payouts WHERE coach_id = coach),
        (SELECT max(returned_at) FROM public.review_requests WHERE coach_id = coach),
        (SELECT max(updated_at) FROM public.coaching_contracts WHERE coach_id = coach AND status IN ('ended', 'refunded')),
        (SELECT max(GREATEST(answered_at, clarified_at, closed_at)) FROM public.coaching_consultations WHERE coach_id = coach))
$$;
REVOKE EXECUTE ON FUNCTION identity_boundary.last_transaction(uuid) FROM PUBLIC;

CREATE TABLE identity_boundary.retirement (
    coach_id uuid PRIMARY KEY,
    ended_at timestamptz NOT NULL,
    last_transaction_at timestamptz
);

CREATE FUNCTION identity_boundary.mark_retired(coach uuid) RETURNS void
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $$
    INSERT INTO identity_boundary.retirement (coach_id, ended_at, last_transaction_at)
    VALUES (coach, statement_timestamp(), identity_boundary.last_transaction(coach))
    ON CONFLICT (coach_id) DO UPDATE
    SET last_transaction_at = GREATEST(retirement.last_transaction_at, EXCLUDED.last_transaction_at)
$$;
REVOKE EXECUTE ON FUNCTION identity_boundary.mark_retired(uuid) FROM PUBLIC;

CREATE FUNCTION identity_boundary.before_profile_removal() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM public.coach_profiles WHERE profile_id = OLD.id) THEN
        PERFORM identity_boundary.mark_retired(OLD.id);
    END IF;
    RETURN OLD;
END;
$$;
CREATE TRIGGER identity_profile_retirement BEFORE DELETE ON public.profiles
FOR EACH ROW EXECUTE FUNCTION identity_boundary.before_profile_removal();
REVOKE EXECUTE ON FUNCTION identity_boundary.before_profile_removal() FROM PUBLIC;

CREATE FUNCTION identity_boundary.record_retirement() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    coach uuid;
BEGIN
    IF TG_OP = 'DELETE' THEN
        coach := OLD.profile_id;
    ELSE
        coach := NEW.profile_id;
        IF NEW.status IN ('approved', 'pending') THEN
            DELETE FROM identity_boundary.retirement WHERE coach_id = coach;
            RETURN NEW;
        END IF;
        IF NEW.status NOT IN ('ended', 'withdrawn', 'rejected') THEN
            RETURN NEW;
        END IF;
    END IF;
    PERFORM identity_boundary.mark_retired(coach);
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER identity_retirement_delete BEFORE DELETE ON public.coach_profiles
FOR EACH ROW EXECUTE FUNCTION identity_boundary.record_retirement();
CREATE TRIGGER identity_retirement_status AFTER UPDATE ON public.coach_profiles
FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION identity_boundary.record_retirement();
REVOKE EXECUTE ON FUNCTION identity_boundary.record_retirement() FROM PUBLIC;

CREATE VIEW identity_boundary.retention AS
SELECT coach_id, ended_at, last_transaction_at
FROM identity_boundary.retirement
WHERE NOT EXISTS (
    SELECT 1 FROM public.coach_profiles cp
    WHERE cp.profile_id = coach_id AND cp.status IN ('approved', 'pending')
);
