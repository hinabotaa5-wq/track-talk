-- 利用者の種目。登録のときにニックネームと一緒に選んでもらう。ケガの集計で、ケガの質問に種目を入れなくて済むようにする。
-- 値は、ケガ部の種目(injury_group_members.event)と同じコード。以前から使っている人は、次に開いたときに選ぶまで空のまま。
ALTER TABLE profiles
    ADD COLUMN event text,
    ADD CONSTRAINT profiles_event_check
    CHECK (event IS NULL OR event IN ('sprint', 'hurdles', 'middle', 'long', 'jumps', 'throws', 'combined', 'race-walk'));
