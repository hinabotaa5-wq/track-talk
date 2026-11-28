-- プレミアムの再設計（本体 #1053 補足 7231984993・補足2 7231991592、運営者決定 2026-11-28）。
-- 決まった5枠の充足をやめ、選手が動画をテーマごとのグループに分けて送る（合計1〜6本。グループ数・グループごとの本数に制限なし）。
-- グループは「テーマのテンプレ（kind）」と「テーマ名（title）」と「何を意識して取り組んでいるか・何が目的か（focus_note、任意）」を持つ。
-- テンプレは レース全体／スタート／後半／練習／その他 の5つ。レース全体のグループの動画だけ、この時のタイム・風速・コンディションを持てる（API で確かめる）。
-- 返却は、グループごとの自由記入1つ（review_response_sections）＋最後の「まとめ」（review_responses.summary）。
-- 000195 は本番未適用（未デプロイ）だが、開発・検証の DB にある行は下で新しい形へ移す。スタンダードの行・流れには触れない。

CREATE TABLE review_request_video_groups (
    request_id uuid NOT NULL REFERENCES review_requests (id) ON DELETE CASCADE,
    position smallint NOT NULL,
    kind text NOT NULL,
    title text NOT NULL,
    focus_note text,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (request_id, position),
    CONSTRAINT review_request_video_groups_position_range CHECK (position BETWEEN 1 AND 6),
    CONSTRAINT review_request_video_groups_kind_check CHECK (kind IN ('race', 'start', 'second_half', 'practice', 'other')),
    CONSTRAINT review_request_video_groups_title_length CHECK (char_length(title) BETWEEN 1 AND 30),
    CONSTRAINT review_request_video_groups_focus_note_length CHECK (focus_note IS NULL OR char_length(focus_note) BETWEEN 1 AND 300)
);

-- 総括の引用単位はグループ名なので、1つの依頼の中でテーマ名は重ねない。
CREATE UNIQUE INDEX review_request_video_groups_title_key ON review_request_video_groups (request_id, title);

ALTER TABLE review_request_video_groups ENABLE ROW LEVEL SECURITY;

-- 既存行の移し替え：枠1つをグループ1つにする（枠の決まった順に並べ、テーマ名は枠名）。練習の動画の目的は、そのグループの意識・目的にまとめる。
INSERT INTO review_request_video_groups (request_id, position, kind, title, focus_note)
SELECT request_id,
       row_number() OVER (PARTITION BY request_id ORDER BY array_position(ARRAY['race', 'start', 'second_half', 'practice', 'other']::text[], slot))::smallint,
       slot,
       CASE slot WHEN 'race' THEN 'レース全体' WHEN 'start' THEN 'スタート' WHEN 'second_half' THEN '後半' WHEN 'practice' THEN '練習' ELSE 'その他' END,
       left(string_agg(practice_purpose, E'\n' ORDER BY position), 300)
FROM review_request_videos
GROUP BY request_id, slot;

ALTER TABLE review_request_videos
    ADD COLUMN group_position smallint;

UPDATE review_request_videos v
SET group_position = g.position
FROM review_request_video_groups g
WHERE g.request_id = v.request_id AND g.kind = v.slot;

ALTER TABLE review_request_videos
    ALTER COLUMN group_position SET NOT NULL,
    ADD CONSTRAINT review_request_videos_group_fkey FOREIGN KEY (request_id, group_position)
        REFERENCES review_request_video_groups (request_id, position) ON DELETE CASCADE;

-- 枠と練習の目的は動画からグループへ移した。レースの入力がどの動画に書けるかはグループの kind で決まるので API で確かめる。
-- 本数の上限は10本から6本へ。既存行（開発 DB に7本目以降があり得る）は確かめず、新しい行から効かせる。
ALTER TABLE review_request_videos
    DROP CONSTRAINT review_request_videos_race_fields,
    DROP CONSTRAINT review_request_videos_practice_fields,
    DROP CONSTRAINT review_request_videos_practice_purpose_length,
    DROP CONSTRAINT review_request_videos_slot_check,
    DROP CONSTRAINT review_request_videos_position_range,
    DROP COLUMN slot,
    DROP COLUMN practice_purpose;

ALTER TABLE review_request_videos
    ADD CONSTRAINT review_request_videos_position_range CHECK (position BETWEEN 1 AND 6) NOT VALID;

-- 返却のグループごとの記入。動画があるグループすべてに1つずつ（API で確かめる）。項目分けはしない自由記入。
ALTER TABLE review_response_sections
    ADD COLUMN group_position smallint;

UPDATE review_response_sections s
SET group_position = g.position
FROM review_request_video_groups g
WHERE g.request_id = s.request_id AND g.kind = s.slot;

-- 移す先のグループが無い記入（動画のない枠に書かれた行。API では作れない）は捨てる。
DELETE FROM review_response_sections WHERE group_position IS NULL;

ALTER TABLE review_response_sections
    DROP CONSTRAINT review_response_sections_pkey,
    DROP CONSTRAINT review_response_sections_slot_check,
    DROP CONSTRAINT review_response_sections_body_length,
    DROP COLUMN slot,
    ALTER COLUMN group_position SET NOT NULL,
    ADD PRIMARY KEY (request_id, group_position),
    ADD CONSTRAINT review_response_sections_group_fkey FOREIGN KEY (request_id, group_position)
        REFERENCES review_request_video_groups (request_id, position) ON DELETE CASCADE,
    ADD CONSTRAINT review_response_sections_body_length CHECK (char_length(body) BETWEEN 1 AND 2000);

-- 最後の「まとめ」。全体分はここに一本化する（旧形の「全体の一言診断」は廃止）。スタンダードでは NULL。
ALTER TABLE review_responses
    ADD COLUMN summary text,
    ADD CONSTRAINT review_responses_summary_length CHECK (summary IS NULL OR char_length(summary) BETWEEN 1 AND 2000);

-- 旧形の総括の一言診断は、まとめへ移す（プレミアムの返答は型の3欄が NULL）。
UPDATE review_responses
SET summary = hitokoto, hitokoto = NULL
WHERE good_point IS NULL AND hitokoto IS NOT NULL;
