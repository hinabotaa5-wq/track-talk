-- 運営が止めていたコーチの登録を、戻さないまま終えられるようにする。ended: 運営が登録を終えた（本人がやめた withdrawn とは分ける）。
-- 終えたコーチは承認済みでないので、残りの報酬は最低額に届かなくても次の月次送金で送る（最終精算）。新しい送金経路は作らない。
ALTER TABLE coach_profiles DROP CONSTRAINT coach_profiles_status_check;
ALTER TABLE coach_profiles ADD CONSTRAINT coach_profiles_status_check
    CHECK (status IN ('pending', 'approved', 'rejected', 'suspended', 'withdrawn', 'ended'));

-- 誰がいつ登録を終えたか。コーチ1人につき1行。
CREATE TABLE coach_registration_endings (
    coach_id uuid PRIMARY KEY REFERENCES coach_profiles (profile_id) ON DELETE CASCADE,
    ended_by uuid REFERENCES profiles (id) ON DELETE SET NULL,
    ended_at timestamptz NOT NULL DEFAULT now()
);
-- API からだけ読み書きする。anon・authenticated からは見えないようにする。
ALTER TABLE coach_registration_endings ENABLE ROW LEVEL SECURITY;
