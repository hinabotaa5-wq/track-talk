-- 公開ページに載る「次の人への一言」を通報・非表示にできるようにする。
-- 通報は uuid で対象を指すので、記録に一言用の uuid を持たせる。
ALTER TABLE injury_group_stage_log
    ADD COLUMN note_id uuid NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN hidden_at timestamptz,
    ADD CONSTRAINT injury_group_stage_log_note_id_key UNIQUE (note_id);

ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check CHECK (target_type IN ('post', 'comment', 'injury_note'));
