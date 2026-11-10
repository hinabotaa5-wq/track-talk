-- ops #63：身元専用サービスのアプリ用ロールが金庫の表を読めるようにする RLS の方針。
--
-- schema.sql は金庫の5表で RLS を有効にし、方針を置いていない。本番のアプリ用ロールは BYPASSRLS を
-- 持たないため（2026-11-11 の金庫 migration ジョブの確認で app_roles_bypass_rls=0・rls_policies=0）、
-- members も identities も0行に見え、身元専用サービスはどの利用者にも 403 を返していた。
--
-- 方針の対象は身元専用サービスのアプリ用ロール（identities への INSERT 権を明示で持つ、ログイン可能で
-- スーパーユーザーでないロール。003 と同じ選び方）だけ。PUBLIC・Supabase の API 用ロール・輸出用ロールには
-- 方針を付けない。何ができるかは従来どおり表の権限（GRANT）で決まり、方針は行を隠さないことだけを足す。
-- members は読み取りだけ（アプリは自分で割り当てを作れない原則のまま）。何度流しても同じ結果になる。
DO $$
DECLARE
    roles text;
    t text;
BEGIN
    SELECT string_agg(DISTINCT format('%I', g.rolname), ', ')
    INTO roles
    FROM pg_class c
    CROSS JOIN LATERAL aclexplode(c.relacl) a
    JOIN pg_roles g ON g.oid = a.grantee
    WHERE c.oid = 'identity_vault.identities'::regclass
      AND a.privilege_type = 'INSERT'
      AND a.grantee <> c.relowner
      AND g.rolcanlogin
      AND NOT g.rolsuper
      AND g.rolname NOT IN ('anon', 'authenticated', 'service_role', 'identity_export');
    FOREACH t IN ARRAY ARRAY['members', 'identities', 'commerce', 'disclosures', 'access_log'] LOOP
        EXECUTE format('DROP POLICY IF EXISTS identity_service_app ON identity_vault.%I', t);
    END LOOP;
    IF roles IS NULL THEN
        -- アプリ用ロールがまだ無い環境（作成前）では何もしない。ジョブの確認で app_roles=0 として出る。
        RETURN;
    END IF;
    EXECUTE format('CREATE POLICY identity_service_app ON identity_vault.members FOR SELECT TO %s USING (true)', roles);
    FOREACH t IN ARRAY ARRAY['identities', 'commerce', 'disclosures', 'access_log'] LOOP
        EXECUTE format('CREATE POLICY identity_service_app ON identity_vault.%I FOR ALL TO %s USING (true) WITH CHECK (true)', t, roles);
    END LOOP;
END
$$;
