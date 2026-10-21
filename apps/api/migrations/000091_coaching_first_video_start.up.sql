-- 期間を「最初の動画が届いた日から」数えるプラン。払ったのに撮影できず日数が減る、を防ぐ。
-- 始まって（承諾・支払い・保護者の同意）から7日のあいだに利用者が動画を送ると、その時から期間を数え直す。
-- 7日以内に届かなければ、8日目から数え始めたのと同じになる（始まった時点で終了日を7日先に延ばしておく）。
ALTER TABLE coaching_plans
    ADD COLUMN start_on_first_video boolean NOT NULL DEFAULT false;

ALTER TABLE coaching_contracts
    -- 申し込んだ時点のプランの写し。
    ADD COLUMN start_on_first_video boolean NOT NULL DEFAULT false,
    -- 最初の動画が届いて、期間を数え始めた時刻。
    ADD COLUMN clock_started_at timestamptz,
    ADD CONSTRAINT coaching_contracts_clock_check CHECK (clock_started_at IS NULL OR start_on_first_video);
