CREATE TABLE injury_group_members (
    user_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    body_part text NOT NULL,
    injured_on date NOT NULL,
    place text NOT NULL,
    situation text NOT NULL,
    stage text NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, body_part),
    CHECK (body_part IN ('foot', 'ankle', 'achilles', 'calf', 'shin', 'knee', 'quad', 'hamstring', 'groin', 'hip', 'lower-back', 'upper-body', 'other')),
    CHECK (stage IN ('resting', 'rehab', 'jogging', 'practice', 'race')),
    CHECK (char_length(btrim(place)) BETWEEN 1 AND 100),
    CHECK (char_length(btrim(situation)) BETWEEN 1 AND 500)
);

CREATE INDEX injury_group_members_part_stage_idx ON injury_group_members (body_part, stage);

ALTER TABLE injury_group_members ENABLE ROW LEVEL SECURITY;

-- 段階を変えた履歴。戻した場合も含めて時系列で残し、再発の振り返りに使う。
CREATE TABLE injury_group_stage_log (
    id bigserial PRIMARY KEY,
    user_id uuid NOT NULL,
    body_part text NOT NULL,
    stage text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (user_id, body_part) REFERENCES injury_group_members (user_id, body_part) ON DELETE CASCADE,
    CHECK (stage IN ('resting', 'rehab', 'jogging', 'practice', 'race'))
);

CREATE INDEX injury_group_stage_log_member_idx ON injury_group_stage_log (user_id, body_part, created_at);

ALTER TABLE injury_group_stage_log ENABLE ROW LEVEL SECURITY;

-- グループで書いた投稿が、投稿した時点でどの段階にいたか。段階を戻しても書き換えない。
ALTER TABLE injury_details
    ADD COLUMN group_stage text,
    ADD CONSTRAINT injury_details_group_stage_check CHECK (group_stage IS NULL OR group_stage IN ('resting', 'rehab', 'jogging', 'practice', 'race'));
