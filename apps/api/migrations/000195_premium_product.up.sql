-- プレミアム（本体 #1053）：決まった5枠（レース全体／スタート／後半／練習／その他）に、申込全体で最大10本の動画を
-- 送り、コーチが総括（全体の一言診断＋枠ごとのブロック）で返す。スタンダードの既存行・既存の流れは変えない。

-- 依頼のプラン。既存行と、プランを指定しない作成はすべてスタンダード。tier はコーチの種別なので流用しない。
ALTER TABLE review_requests
    ADD COLUMN plan text NOT NULL DEFAULT 'standard';

ALTER TABLE review_requests
    ADD CONSTRAINT review_requests_plan_check CHECK (plan IN ('standard', 'premium'));

-- プレミアムには相談内容の自由記述も「見てほしい点」もない（枠と動画ごとの入力が代わり）。
-- スタンダードは従来どおり、相談内容か見てほしい点のどちらかが要る。
ALTER TABLE review_requests
    DROP CONSTRAINT review_requests_question_or_focus_check;

ALTER TABLE review_requests
    ADD CONSTRAINT review_requests_question_or_focus_check CHECK (plan = 'premium' OR char_length(btrim(question)) > 0 OR cardinality(focus) > 0);

-- プレミアムの動画。1行が動画1本。枠ごとの入力は、レース全体の動画だけ「この時のタイム・風速・その時の体のコンディション」、
-- 練習の動画だけ「その練習をした目的」を持つ（すべて任意。未入力は NULL）。並び順は申込で選手が追加した順。
-- 動画 ID は他の会話の動画と同じく一意（使い回し・他人の動画の流用を防ぐ）。保持期限・退会での削除は依頼の動画と同じ扱い。
CREATE TABLE review_request_videos (
    request_id uuid NOT NULL REFERENCES review_requests (id) ON DELETE CASCADE,
    position smallint NOT NULL,
    slot text NOT NULL,
    video_public_id text NOT NULL UNIQUE,
    race_time text,
    wind_speed text,
    body_condition text,
    practice_purpose text,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (request_id, position),
    CONSTRAINT review_request_videos_position_range CHECK (position BETWEEN 1 AND 10),
    CONSTRAINT review_request_videos_slot_check CHECK (slot IN ('race', 'start', 'second_half', 'practice', 'other')),
    CONSTRAINT review_request_videos_race_fields CHECK (slot = 'race' OR (race_time IS NULL AND wind_speed IS NULL AND body_condition IS NULL)),
    CONSTRAINT review_request_videos_practice_fields CHECK (slot = 'practice' OR practice_purpose IS NULL),
    CONSTRAINT review_request_videos_race_time_length CHECK (race_time IS NULL OR char_length(race_time) BETWEEN 1 AND 100),
    CONSTRAINT review_request_videos_wind_speed_length CHECK (wind_speed IS NULL OR char_length(wind_speed) BETWEEN 1 AND 50),
    CONSTRAINT review_request_videos_body_condition_length CHECK (body_condition IS NULL OR char_length(body_condition) BETWEEN 1 AND 300),
    CONSTRAINT review_request_videos_practice_purpose_length CHECK (practice_purpose IS NULL OR char_length(practice_purpose) BETWEEN 1 AND 300)
);

ALTER TABLE review_request_videos ENABLE ROW LEVEL SECURITY;

-- プレミアムの返却（総括）は、全体の一言診断（review_responses.hitokoto）と枠ごとのブロック（下の表）で書く。
-- スタンダードの型の欄（良い点・ボトルネック・ドリル）はプレミアムでは使わないので NULL を許す。
-- 3欄は「全部ある（スタンダード）」か「全部ない（プレミアム）」のどちらか。既存行はすべて全部ある。
ALTER TABLE review_responses
    ALTER COLUMN good_point DROP NOT NULL,
    ALTER COLUMN fix_points DROP NOT NULL,
    ALTER COLUMN drill DROP NOT NULL;

ALTER TABLE review_responses
    ADD CONSTRAINT review_responses_standard_fields CHECK (
        (good_point IS NULL) = (fix_points IS NULL) AND (good_point IS NULL) = (drill IS NULL)
    );

-- 総括の枠ごとのブロック。動画がある枠についてだけ書く（API で確かめる）。枠名が見出しで、根拠の引用単位も枠名。
CREATE TABLE review_response_sections (
    request_id uuid NOT NULL REFERENCES review_responses (request_id) ON DELETE CASCADE,
    slot text NOT NULL,
    body text NOT NULL,
    PRIMARY KEY (request_id, slot),
    CONSTRAINT review_response_sections_slot_check CHECK (slot IN ('race', 'start', 'second_half', 'practice', 'other')),
    CONSTRAINT review_response_sections_body_length CHECK (char_length(body) BETWEEN 1 AND 1000)
);

ALTER TABLE review_response_sections ENABLE ROW LEVEL SECURITY;
