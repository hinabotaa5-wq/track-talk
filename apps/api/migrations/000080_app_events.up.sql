-- アプリ内の種目を、コーチの担当種目に統一する。リレー(relay)は、コーチの担当種目と練習相手募集だけに入れる。
-- 練習相手募集・感覚メモ・競技会の記録・レビューの種目に、競歩(race-walk)を足す。
-- 以前に保存した値(七種・十種・4x100・4x400・その他)はそのまま残すので、制約には残しておく。

-- 利用者の種目。登録のときにニックネームと一緒に選んでもらう。以前から使っている人は、次に開いたときに選ぶまで空のまま。
ALTER TABLE profiles
    ADD COLUMN event text,
    ADD CONSTRAINT profiles_event_check CHECK (
        event IS NULL OR event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh',
            '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump',
            'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk'
        )
    );

ALTER TABLE practice_details
    DROP CONSTRAINT practice_details_event_check,
    ADD CONSTRAINT practice_details_event_check CHECK (
        event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh',
            '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump',
            'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk',
            'heptathlon', 'decathlon', '4x100', '4x400', 'relay', 'other'
        )
    );

ALTER TABLE sense_notes
    DROP CONSTRAINT sense_notes_event_check,
    ADD CONSTRAINT sense_notes_event_check CHECK (
        event IS NULL OR event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh',
            '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump',
            'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk',
            'heptathlon', 'decathlon', '4x100', '4x400', 'other'
        )
    );

ALTER TABLE product_reviews
    DROP CONSTRAINT product_reviews_event_check,
    ADD CONSTRAINT product_reviews_event_check CHECK (
        event IS NULL OR event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh',
            '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump',
            'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk',
            'heptathlon', 'decathlon', '4x100', '4x400', 'other'
        )
    );

ALTER TABLE race_records
    DROP CONSTRAINT race_records_event_check,
    ADD CONSTRAINT race_records_event_check CHECK (
        event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh',
            '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump',
            'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk',
            'heptathlon', 'decathlon', '4x100', '4x400'
        )
    );

-- ケガ部の種目は、登録した種目をそのまま使う。以前の区分(短距離・ハードルなど)で答えた人の値も残す。
ALTER TABLE injury_group_members
    DROP CONSTRAINT injury_group_members_event_check,
    ADD CONSTRAINT injury_group_members_event_check CHECK (
        event IS NULL OR event IN (
            '100m', '200m', '400m', '800m', '1500m', '5000m', '10000m', '110mh',
            '100mh', '400mh', '3000msc', 'high-jump', 'pole-vault', 'long-jump',
            'triple-jump', 'shot-put', 'discus', 'hammer', 'javelin', 'race-walk',
            'sprint', 'hurdles', 'middle', 'long', 'jumps', 'throws', 'combined'
        )
    );
