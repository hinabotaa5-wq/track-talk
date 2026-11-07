-- 新しいコーチが加わったときの全ユーザーへの自動通知（本体 #438）。
-- コーチごとの発火済み記録。停止→再開・見送り後の再承認・プロフィール編集では再送しない。
-- デプロイ時点で既に申請済み（審査待ち以外）のコーチは発火済みとして初期値を入れ、遡って送らない。
CREATE TABLE new_coach_announcements (
    coach_id uuid PRIMARY KEY REFERENCES profiles (id) ON DELETE CASCADE,
    fired_at timestamptz NOT NULL DEFAULT now(),
    notified boolean NOT NULL DEFAULT false,
    announcement_id uuid REFERENCES announcements (id) ON DELETE SET NULL
);

ALTER TABLE new_coach_announcements ENABLE ROW LEVEL SECURITY;

INSERT INTO new_coach_announcements (coach_id)
SELECT profile_id FROM coach_profiles WHERE status <> 'pending'
ON CONFLICT DO NOTHING;
