-- ops #63：合成データ専用のプロビジョニング（provision／verify／cleanup）のための狭い関数。
--
-- 金庫の migration 所有者で、002 の後に適用する（コマンド経路：Cloud Run Job track-talk-identity-vault-migrate）。
-- 何度流しても同じ結果になる（CREATE OR REPLACE と付与のやり直しだけ）。既存の表・行・RLS には触れない。
--
-- アプリ（身元専用サービス）の DB ロールは members を読めるが自分で割り当てを作れない（schema.sql）。
-- この migration はその原則を保ったまま、次の条件をすべて満たす合成の割り当てだけを作れ・消せるようにする。
--   - subject（金庫 Auth のユーザー）のメールの @ より前に「+synthetic」が含まれる（合成の標識）
--   - is_admin は常に false。管理者行は作らない・変えない・消さない
--   - 既に合成でない subject や管理者に結び付いたコーチ ID、合成でない行には触れない
-- 本体側のガード（入力コーチ ID のアカウントが承認済みで、メールが入力と一致する）は、実行体が本体の
-- identity_boundary.coach_matches_email で先に確かめる。金庫側はメールの値を返さず、真偽と件数だけを返す。

-- 合成の標識。メールの値そのものは返さない。
CREATE OR REPLACE FUNCTION identity_vault.synthetic_subject(target uuid) RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM auth.users u
        WHERE u.id = target
          AND u.deleted_at IS NULL
          AND u.email IS NOT NULL
          AND position('+synthetic' IN lower(split_part(u.email, '@', 1))) > 0);
END;
$$;

-- 合成の割り当てを作る。既にあれば何もしない。返り値は created／exists。
-- 条件に合わないときは例外（SQLSTATE 'P0001'、メッセージは固定の分類名）で止まり、何も書かない。
CREATE OR REPLACE FUNCTION identity_vault.synthetic_bind(target_subject uuid, target_coach uuid) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    existing_coach uuid;
    existing_admin boolean;
    holder uuid;
BEGIN
    IF target_subject IS NULL OR target_coach IS NULL THEN
        RAISE EXCEPTION 'synthetic_invalid_input';
    END IF;
    IF NOT identity_vault.synthetic_subject(target_subject) THEN
        RAISE EXCEPTION 'synthetic_subject_required';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(target_coach::text, 0));
    SELECT coach_id, is_admin INTO existing_coach, existing_admin
    FROM identity_vault.members WHERE subject = target_subject FOR UPDATE;
    IF FOUND THEN
        IF existing_admin OR existing_coach IS DISTINCT FROM target_coach THEN
            RAISE EXCEPTION 'synthetic_subject_bound_elsewhere';
        END IF;
        RETURN 'exists';
    END IF;
    SELECT subject INTO holder FROM identity_vault.members WHERE coach_id = target_coach FOR UPDATE;
    IF FOUND THEN
        -- 別の subject が既にこのコーチに結び付いている。合成どうしでも自動の付け替えはしない
        -- （先に cleanup で外す）。合成でない subject なら実データなので触れない。
        RAISE EXCEPTION 'synthetic_coach_bound_elsewhere';
    END IF;
    INSERT INTO identity_vault.members (subject, coach_id, is_admin) VALUES (target_subject, target_coach, false);
    RETURN 'created';
END;
$$;

-- 割り当ての状態を真偽と件数だけで返す（subject の値・メールは返さない）。
-- expected_subject は実行体が金庫 Auth から得た合成ユーザーの ID で、members の subject と一致するかを見る。
CREATE OR REPLACE FUNCTION identity_vault.synthetic_status(target_coach uuid, expected_subject uuid)
RETURNS TABLE (member_rows integer, subject_matches boolean, subject_synthetic boolean, admin_rows integer,
               identity_rows integer, commerce_rows integer, disclosure_rows integer, access_log_rows integer,
               identity_ended boolean)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    bound uuid;
BEGIN
    SELECT m.subject INTO bound FROM identity_vault.members m WHERE m.coach_id = target_coach;
    member_rows := (SELECT count(*) FROM identity_vault.members m WHERE m.coach_id = target_coach)::integer;
    subject_matches := bound IS NOT NULL AND bound = expected_subject;
    subject_synthetic := bound IS NOT NULL AND identity_vault.synthetic_subject(bound);
    admin_rows := (SELECT count(*) FROM identity_vault.members m
                   WHERE (m.coach_id = target_coach OR m.subject = expected_subject) AND m.is_admin)::integer;
    identity_rows := (SELECT count(*) FROM identity_vault.identities i WHERE i.coach_id = target_coach)::integer;
    commerce_rows := (SELECT count(*) FROM identity_vault.commerce c WHERE c.coach_id = target_coach)::integer;
    disclosure_rows := (SELECT count(*) FROM identity_vault.disclosures d WHERE d.coach_id = target_coach)::integer;
    access_log_rows := (SELECT count(*) FROM identity_vault.access_log l WHERE l.coach_id = target_coach)::integer;
    identity_ended := EXISTS (SELECT 1 FROM identity_vault.identities i WHERE i.coach_id = target_coach AND i.ended_at IS NOT NULL);
    RETURN NEXT;
END;
$$;

-- 合成の割り当てと、そのコーチの合成の身元行（identities・commerce・disclosures）を消す。
-- members 行の subject が合成で is_admin=false のときだけ動く。access_log は監査のため残す。
-- 返り値は消した行の件数と、外した合成 subject（実行体が金庫 Auth の合成ユーザーを消すためだけに使い、出力しない）。
-- 割り当てが無ければ何もしない（0件）。
CREATE OR REPLACE FUNCTION identity_vault.synthetic_unbind(target_coach uuid)
RETURNS TABLE (member_rows integer, identity_rows integer, commerce_rows integer, disclosure_rows integer, unbound_subject uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    bound uuid;
    bound_admin boolean;
BEGIN
    member_rows := 0; identity_rows := 0; commerce_rows := 0; disclosure_rows := 0;
    IF target_coach IS NULL THEN
        RAISE EXCEPTION 'synthetic_invalid_input';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(target_coach::text, 0));
    SELECT m.subject, m.is_admin INTO bound, bound_admin FROM identity_vault.members m WHERE m.coach_id = target_coach FOR UPDATE;
    unbound_subject := NULL;
    IF NOT FOUND THEN
        RETURN NEXT;
        RETURN;
    END IF;
    IF bound_admin OR NOT identity_vault.synthetic_subject(bound) THEN
        RAISE EXCEPTION 'synthetic_subject_required';
    END IF;
    DELETE FROM identity_vault.disclosures d WHERE d.coach_id = target_coach;
    GET DIAGNOSTICS disclosure_rows = ROW_COUNT;
    DELETE FROM identity_vault.identities i WHERE i.coach_id = target_coach;
    GET DIAGNOSTICS identity_rows = ROW_COUNT;
    DELETE FROM identity_vault.commerce c WHERE c.coach_id = target_coach;
    GET DIAGNOSTICS commerce_rows = ROW_COUNT;
    DELETE FROM identity_vault.members m WHERE m.coach_id = target_coach AND m.subject = bound AND NOT m.is_admin;
    GET DIAGNOSTICS member_rows = ROW_COUNT;
    unbound_subject := bound;
    RETURN NEXT;
END;
$$;

-- 管理者側の参照の前提を、値を見ずに件数だけで確かめる（管理者の実ログインの代わり）。
CREATE OR REPLACE FUNCTION identity_vault.admin_readiness()
RETURNS TABLE (admin_rows integer, admins_with_verified_mfa integer)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    admin_rows := (SELECT count(*) FROM identity_vault.members m WHERE m.is_admin)::integer;
    admins_with_verified_mfa := (SELECT count(*) FROM identity_vault.members m
        WHERE m.is_admin AND EXISTS (SELECT 1 FROM auth.mfa_factors f WHERE f.user_id = m.subject AND f.status = 'verified'))::integer;
    RETURN NEXT;
END;
$$;

-- 一回限り：運営者決定（2026-11-10 22:53、ops #63）で、合成の標識を満たさない状態でも削除を許可された
-- 旧「aaa」の割り当て1件だけを外す。対象は本体コーチ ID が 2877c3f1- で始まる、管理者でない members 行で、
-- 1件に定まらなければ何もせず止まる。そのコーチの身元行も消す。access_log は残す。
-- 返す subject は、実行体がその金庫 Auth ユーザー（TOTP の設定キーが露出したもの）を消すためだけに使う。
CREATE OR REPLACE FUNCTION identity_vault.legacy_unbind_aaa()
RETURNS TABLE (matched_rows integer, legacy_coach uuid, member_rows integer, identity_rows integer, commerce_rows integer,
               disclosure_rows integer, unbound_subject uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    bound uuid;
BEGIN
    matched_rows := 0; member_rows := 0; identity_rows := 0; commerce_rows := 0; disclosure_rows := 0;
    legacy_coach := NULL; unbound_subject := NULL;
    SELECT count(*)::integer INTO matched_rows FROM identity_vault.members m
    WHERE m.coach_id::text LIKE '2877c3f1-%' AND NOT m.is_admin;
    IF matched_rows = 0 THEN
        RETURN NEXT;
        RETURN;
    END IF;
    IF matched_rows > 1 THEN
        RAISE EXCEPTION 'legacy_ambiguous';
    END IF;
    SELECT m.coach_id, m.subject INTO legacy_coach, bound FROM identity_vault.members m
    WHERE m.coach_id::text LIKE '2877c3f1-%' AND NOT m.is_admin FOR UPDATE;
    IF EXISTS (SELECT 1 FROM identity_vault.members m WHERE m.subject = bound AND m.is_admin) THEN
        RAISE EXCEPTION 'legacy_subject_is_admin';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended(legacy_coach::text, 0));
    DELETE FROM identity_vault.disclosures d WHERE d.coach_id = legacy_coach;
    GET DIAGNOSTICS disclosure_rows = ROW_COUNT;
    DELETE FROM identity_vault.identities i WHERE i.coach_id = legacy_coach;
    GET DIAGNOSTICS identity_rows = ROW_COUNT;
    DELETE FROM identity_vault.commerce c WHERE c.coach_id = legacy_coach;
    GET DIAGNOSTICS commerce_rows = ROW_COUNT;
    DELETE FROM identity_vault.members m WHERE m.coach_id = legacy_coach AND m.subject = bound AND NOT m.is_admin;
    GET DIAGNOSTICS member_rows = ROW_COUNT;
    unbound_subject := bound;
    RETURN NEXT;
END;
$$;

REVOKE ALL ON FUNCTION identity_vault.synthetic_subject(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION identity_vault.legacy_unbind_aaa() FROM PUBLIC;
REVOKE ALL ON FUNCTION identity_vault.synthetic_bind(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION identity_vault.synthetic_status(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION identity_vault.synthetic_unbind(uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION identity_vault.admin_readiness() FROM PUBLIC;

-- 実行を許すのは身元専用サービスのアプリ用ロール（identities に書き込めるログイン可能なロール）だけ。
-- 本番のロール名は運営者が決めて作成したため、名前ではなく付与の実物で選ぶ。輸出用の読み取り専用ロール・
-- Supabase の API 用ロール・スーパーユーザー・所有者は対象外。
DO $$
DECLARE
    r text;
    fn text;
BEGIN
    FOREACH r IN ARRAY ARRAY['anon', 'authenticated', 'service_role', 'identity_export'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r) THEN
            FOREACH fn IN ARRAY ARRAY['identity_vault.synthetic_subject(uuid)', 'identity_vault.synthetic_bind(uuid, uuid)',
                                      'identity_vault.synthetic_status(uuid, uuid)', 'identity_vault.synthetic_unbind(uuid)',
                                      'identity_vault.admin_readiness()', 'identity_vault.legacy_unbind_aaa()'] LOOP
                EXECUTE format('REVOKE ALL ON FUNCTION %s FROM %I', fn, r);
            END LOOP;
        END IF;
    END LOOP;
    FOR r IN
        SELECT DISTINCT g.rolname
        FROM pg_class c
        CROSS JOIN LATERAL aclexplode(c.relacl) a
        JOIN pg_roles g ON g.oid = a.grantee
        WHERE c.oid = 'identity_vault.identities'::regclass
          AND a.privilege_type = 'INSERT'
          AND a.grantee <> c.relowner
          AND g.rolcanlogin
          AND NOT g.rolsuper
          AND g.rolname NOT IN ('anon', 'authenticated', 'service_role', 'identity_export')
    LOOP
        EXECUTE format('GRANT EXECUTE ON FUNCTION identity_vault.synthetic_bind(uuid, uuid), identity_vault.synthetic_status(uuid, uuid), '
                       'identity_vault.synthetic_unbind(uuid), identity_vault.admin_readiness(), identity_vault.legacy_unbind_aaa() TO %I', r);
    END LOOP;
END
$$;
