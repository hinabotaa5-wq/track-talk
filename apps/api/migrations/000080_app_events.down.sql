-- 新しく足した種目(七種・十種)をコーチの側から外す。
DELETE FROM review_requests WHERE event IN ('heptathlon', 'decathlon');
UPDATE coach_profiles SET events = ARRAY(SELECT e FROM unnest(events) e WHERE e NOT IN ('heptathlon', 'decathlon'));
DELETE FROM coach_profiles WHERE cardinality(events) = 0;
ALTER TABLE coach_profiles
    DROP CONSTRAINT coach_profiles_events_check,
    ADD CONSTRAINT coach_profiles_events_check CHECK (
        cardinality(events) BETWEEN 1 AND 21
        AND events <@ ARRAY[
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh',
            '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump',
            'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk', 'relay'
        ]::text[]
    );
ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_event_check,
    ADD CONSTRAINT review_requests_event_check CHECK (
        event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh',
            '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump',
            'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk', 'relay'
        )
    );

ALTER TABLE injury_group_members
    DROP CONSTRAINT injury_group_members_event_check,
    ADD CONSTRAINT injury_group_members_event_check CHECK (
        event IS NULL OR event IN (
            'sprint', 'hurdles', 'middle', 'long', 'jumps', 'throws', 'combined',
            'race-walk'
        )
    );

ALTER TABLE race_records
    DROP CONSTRAINT race_records_event_check,
    ADD CONSTRAINT race_records_event_check CHECK (
        event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '100mh',
            '110mh', '400mh', '3000msc', 'long-jump', 'triple-jump', 'high-jump',
            'pole-vault', 'shot-put', 'discus', 'hammer', 'javelin', 'heptathlon',
            'decathlon', '4x100', '4x400'
        )
    );

ALTER TABLE product_reviews
    DROP CONSTRAINT product_reviews_event_check,
    ADD CONSTRAINT product_reviews_event_check CHECK (
        event IS NULL OR event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '100mh',
            '110mh', '400mh', '3000msc', 'long-jump', 'triple-jump', 'high-jump',
            'pole-vault', 'shot-put', 'discus', 'hammer', 'javelin', 'heptathlon',
            'decathlon', '4x100', '4x400', 'other'
        )
    );

ALTER TABLE sense_notes
    DROP CONSTRAINT sense_notes_event_check,
    ADD CONSTRAINT sense_notes_event_check CHECK (
        event IS NULL OR event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '100mh',
            '110mh', '400mh', '3000msc', 'long-jump', 'triple-jump', 'high-jump',
            'pole-vault', 'shot-put', 'discus', 'hammer', 'javelin', 'heptathlon',
            'decathlon', '4x100', '4x400', 'other'
        )
    );

ALTER TABLE practice_details
    DROP CONSTRAINT practice_details_event_check,
    ADD CONSTRAINT practice_details_event_check CHECK (
        event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '100mh',
            '110mh', '400mh', '3000msc', 'long-jump', 'triple-jump', 'high-jump',
            'pole-vault', 'shot-put', 'discus', 'hammer', 'javelin', 'heptathlon',
            'decathlon', '4x100', '4x400', 'other'
        )
    );

ALTER TABLE profiles
    DROP CONSTRAINT profiles_event_check,
    DROP COLUMN event;
