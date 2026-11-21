ALTER TABLE coaching_consultations ADD COLUMN offered_at timestamptz;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM coaching_consultations
        WHERE status IN ('offered','in_review','answered','declined')
          AND respond_by IS NULL AND paid_at IS NULL
    ) THEN
        RAISE EXCEPTION 'Cannot backfill consultation offered_at: a delivered consultation has no respond_by or paid_at';
    END IF;
END;
$$;

-- The historic value is a delivery marker, not a precise delivery timestamp.
UPDATE coaching_consultations
SET offered_at = COALESCE(respond_by, paid_at)
WHERE status IN ('offered','in_review','answered','declined');
