-- コーチが自分でコーチをやめられるようにする。withdrawn: 本人がやめた（運営が止めた suspended とは分ける）。
-- アカウントは残り、一般の利用者として使い続けられる。もう一度応募すれば審査からやり直す。
ALTER TABLE coach_profiles DROP CONSTRAINT coach_profiles_status_check;
ALTER TABLE coach_profiles ADD CONSTRAINT coach_profiles_status_check
    CHECK (status IN ('pending', 'approved', 'rejected', 'suspended', 'withdrawn'));
