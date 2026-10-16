-- 段階を変えるたびに、その間に意識的にやったことと、次の人への一言を残す。
-- 入ったときの記録(最初の段階)には、どちらもない。
ALTER TABLE injury_group_stage_log
    ADD COLUMN actions text[] NOT NULL DEFAULT '{}',
    ADD COLUMN note text,
    ADD COLUMN note_public boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT injury_group_stage_log_actions_check CHECK (
        actions <@ ARRAY['hospital-rehab', 'osteopath', 'stretch', 'bath', 'strength', 'icing', 'taping', 'rest', 'nothing']::text[]
    ),
    ADD CONSTRAINT injury_group_stage_log_note_check CHECK (note IS NULL OR char_length(btrim(note)) BETWEEN 1 AND 300),
    ADD CONSTRAINT injury_group_stage_log_note_public_check CHECK (note IS NOT NULL OR NOT note_public);
