DO $$
DECLARE
    constraint_name text;
BEGIN
    SELECT conname INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = 'recruitment_links'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%position%';
    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE recruitment_links DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE recruitment_links
    ADD CONSTRAINT recruitment_links_position_check CHECK (position >= 0 AND position < 4);
