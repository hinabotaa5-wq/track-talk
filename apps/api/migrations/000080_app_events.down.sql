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
