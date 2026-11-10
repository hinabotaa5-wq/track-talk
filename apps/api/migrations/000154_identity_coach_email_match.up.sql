-- 合成データ専用のプロビジョニング（ops #63）のガード「入力コーチ ID のアカウントが承認済みで、
-- そのメールが入力メールと一致する」を、身元専用サービスの本体読み取りロールから確かめられるようにする。
-- メールは auth.users にしかなく、読み取りロールに auth を開けないため、postgres 所有の関数を経由する
-- （public.mfa_verified〔000153〕と同じ型）。返すのは真偽だけで、メールその他の列は返さない。
-- plpgsql にしているのは、auth.users の無い素の Postgres（手元・CI）でも作成できるようにするため。
CREATE FUNCTION identity_boundary.coach_matches_email(target_coach uuid, target_email text) RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF target_coach IS NULL OR target_email IS NULL OR btrim(target_email) = '' THEN
        RETURN false;
    END IF;
    RETURN EXISTS (
        SELECT 1
        FROM public.coach_profiles c
        JOIN auth.users u ON u.id = c.profile_id
        WHERE c.profile_id = target_coach
          AND c.status = 'approved'
          AND u.deleted_at IS NULL
          AND u.email IS NOT NULL
          AND lower(u.email) = lower(btrim(target_email)));
END;
$$;

REVOKE ALL ON FUNCTION identity_boundary.coach_matches_email(uuid, text) FROM PUBLIC;
DO $$
DECLARE
    r text;
BEGIN
    -- Supabase が既定で関数に付ける API 用ロールと、公開 API の実行ロールからは外す（ロールがある環境だけ）。
    FOREACH r IN ARRAY ARRAY['anon', 'authenticated', 'service_role', 'track_talk_api'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r) THEN
            EXECUTE format('REVOKE ALL ON FUNCTION identity_boundary.coach_matches_email(uuid, text) FROM %I', r);
        END IF;
    END LOOP;
    -- 実行を許すのは身元専用サービスの本体読み取りロールだけ。本番ではロール名を運営者が決めて作成し、
    -- identity_boundary.mark_retired の実行権を明示で付与している（配備時の H4）。その明示の付与を持つ
    -- ログイン可能なロールにだけ、同じく明示で付与する。所有者・スーパーユーザー・API 用ロールは対象外。
    FOR r IN
        SELECT DISTINCT g.rolname
        FROM pg_proc p
        CROSS JOIN LATERAL aclexplode(p.proacl) a
        JOIN pg_roles g ON g.oid = a.grantee
        WHERE p.oid = 'identity_boundary.mark_retired(uuid)'::regprocedure
          AND a.privilege_type = 'EXECUTE'
          AND a.grantee <> p.proowner
          AND g.rolcanlogin
          AND NOT g.rolsuper
          AND g.rolname NOT IN ('anon', 'authenticated', 'service_role', 'track_talk_api')
    LOOP
        EXECUTE format('GRANT EXECUTE ON FUNCTION identity_boundary.coach_matches_email(uuid, text) TO %I', r);
    END LOOP;
END
$$;
