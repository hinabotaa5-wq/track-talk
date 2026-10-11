DROP INDEX IF EXISTS competitions_prefecture_starts_on_idx;
DROP INDEX IF EXISTS competitions_identity_idx;

ALTER TABLE competitions
    DROP CONSTRAINT IF EXISTS competitions_prefecture_chk,
    DROP CONSTRAINT IF EXISTS competitions_season_year_chk,
    DROP COLUMN IF EXISTS source_url,
    DROP COLUMN IF EXISTS organizer_url,
    DROP COLUMN IF EXISTS season_year,
    DROP COLUMN IF EXISTS prefecture,
    ADD COLUMN IF NOT EXISTS scope text,
    ADD COLUMN IF NOT EXISTS summary text NOT NULL DEFAULT '';

UPDATE competitions SET scope = 'domestic' WHERE scope IS NULL;

ALTER TABLE competitions
    ALTER COLUMN scope SET NOT NULL;

ALTER TABLE competitions
    DROP CONSTRAINT IF EXISTS competitions_scope_chk;

ALTER TABLE competitions
    ADD CONSTRAINT competitions_scope_chk CHECK (scope IN ('domestic', 'international'));

CREATE INDEX IF NOT EXISTS competitions_starts_on_idx ON competitions (starts_on DESC);

INSERT INTO competitions (id, name, starts_on, ends_on, location, scope, summary) VALUES
    ('11111111-1111-4111-8111-111111111111', '世界陸上競技選手権大会', '2025-09-13', '2025-09-21', '東京', 'international', '世界陸連が主催する世界選手権。種目横断でトップ選手が集まる。'),
    ('22222222-2222-4222-8222-222222222222', 'ダイヤモンドリーグ', '2026-05-01', '2026-09-30', '各地', 'international', '世界陸連公認のトップミーティングシリーズ。'),
    ('33333333-3333-4333-8333-333333333333', '日本陸上競技選手権大会', '2026-06-26', '2026-06-28', '国立競技場', 'domestic', '日本選手権。国内の第一人者が集う主要大会。'),
    ('44444444-4444-4444-8444-444444444444', '日本選手権リレー', '2026-10-01', '2026-10-02', '横浜', 'domestic', 'リレー種目に特化した国内主要大会。')
ON CONFLICT (id) DO NOTHING;
