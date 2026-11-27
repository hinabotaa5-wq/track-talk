-- コーチの添削項目（追加項目）と見本動画の変更申請（本体 #1086、運営者決定 2026-11-27）。
-- 対象はスタンダードだけ。固定の4項目（良い点／改善点／おすすめのドリル／コーチなら明日から何をするか）は全コーチ共通で、
-- コーチが増減できるのは「追加項目（最大5件・各30字）」と「コーチ詳細に常設する見本動画（最大3本）」だけ。
-- 変更は価格申請（000194）と同じく申請制：承認しても即時には切り替えず、承認の時点でそのコーチが対応中だった依頼
-- （承認日時以前に作られ、awaiting_payment／queued／offered／in_review／needs_info のもの）がすべて終わってから有効にする。
-- 既存コーチには行を作らない（追加項目なし・見本動画なしとして扱う）。

-- 申請の履歴。1件が「追加項目の希望セット（全件）＋見本動画の希望セット（全件）」。
-- pending: 承認待ち（コーチが上書き・取り下げできる）。approved: 承認済みで切り替え待ち。applied: 有効になった。
-- withdrawn: コーチが取り下げた。superseded: 切り替え前に、より新しい申請が承認された。却下はしない方針なので rejected は置かない。
CREATE TABLE coach_offer_applications (
    id uuid PRIMARY KEY,
    coach_id uuid NOT NULL REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    -- 追加項目のタイトル（登録順）。空配列は「追加項目なし」。
    items text[] NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'applied', 'withdrawn', 'superseded')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    approved_at timestamptz,
    approved_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    applied_at timestamptz,
    withdrawn_at timestamptz,
    CONSTRAINT coach_offer_applications_items_count CHECK (cardinality(items) <= 5 AND array_position(items, NULL) IS NULL),
    CHECK ((status IN ('pending', 'withdrawn')) = (approved_at IS NULL)),
    CHECK ((status = 'applied') = (applied_at IS NOT NULL)),
    CHECK ((status = 'withdrawn') = (withdrawn_at IS NOT NULL))
);

-- 承認待ち・切り替え待ちはコーチごとに1件だけ（上書きは承認待ちの1件の書き換え）。
CREATE UNIQUE INDEX coach_offer_applications_pending_idx ON coach_offer_applications (coach_id) WHERE status = 'pending';
CREATE UNIQUE INDEX coach_offer_applications_approved_idx ON coach_offer_applications (coach_id) WHERE status = 'approved';
CREATE INDEX coach_offer_applications_coach_idx ON coach_offer_applications (coach_id, created_at DESC);

-- 申請の見本動画（提出した順）。回答ごとの見本動画（review_responses.demo_video_public_id）とは別の保持で、
-- 同じ動画を両方に使える。セットから外しても動画ファイルや回答の見本動画は消さない。
CREATE TABLE coach_offer_application_videos (
    application_id uuid NOT NULL REFERENCES coach_offer_applications (id) ON DELETE CASCADE,
    position smallint NOT NULL CHECK (position BETWEEN 0 AND 2),
    video_public_id text NOT NULL,
    PRIMARY KEY (application_id, position),
    UNIQUE (application_id, video_public_id)
);

-- いま有効な内容（コーチごとに1行）。有効な追加項目・見本動画は、この行が指す申請（applied）のセット。
-- 行がないコーチは追加項目なし・見本動画なし。
CREATE TABLE coach_offers (
    coach_id uuid PRIMARY KEY REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    application_id uuid NOT NULL REFERENCES coach_offer_applications (id) ON DELETE CASCADE,
    effective_at timestamptz NOT NULL DEFAULT now()
);

-- 依頼の作成時に固定した追加項目のタイトル（登録順）。base_price_yen と同じく、後から有効セットが変わっても変えない。
-- 指名したコーチのいるスタンダードの依頼だけが対象で、それ以外（コーチ未定・プレミアム・既存の依頼）は空。
ALTER TABLE review_requests
    ADD COLUMN offer_items text[] NOT NULL DEFAULT '{}',
    ADD CONSTRAINT review_requests_offer_items_count CHECK (cardinality(offer_items) <= 5 AND array_position(offer_items, NULL) IS NULL);

-- 回答の追加項目の本文（依頼に固定したタイトルの順）。固定項目と同じく各300字・必須。
CREATE TABLE review_response_items (
    request_id uuid NOT NULL REFERENCES review_responses (request_id) ON DELETE CASCADE,
    position smallint NOT NULL CHECK (position BETWEEN 0 AND 4),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 30),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 300),
    PRIMARY KEY (request_id, position)
);

-- 回収が決まった動画ファイルを付け直させない（000100 と同じ守り）。
CREATE TRIGGER guard_retired_media BEFORE INSERT OR UPDATE OF video_public_id ON coach_offer_application_videos
    FOR EACH ROW EXECUTE FUNCTION guard_retired_cloudinary_media('video_public_id');

ALTER TABLE coach_offer_applications ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_offer_application_videos ENABLE ROW LEVEL SECURITY;
ALTER TABLE coach_offers ENABLE ROW LEVEL SECURITY;
ALTER TABLE review_response_items ENABLE ROW LEVEL SECURITY;
