-- コーチプロフィール翻訳の予約枠を、全体に加えてコーチごとにも数える。
-- 既存行の coach_id は NULL のまま残し、全体上限の集計にだけ数える（コーチ別集計には含めない）。
ALTER TABLE coach_profile_translation_attempts ADD COLUMN coach_id uuid;
CREATE INDEX coach_profile_translation_attempts_coach_attempted_at_idx ON coach_profile_translation_attempts (coach_id, attempted_at);
