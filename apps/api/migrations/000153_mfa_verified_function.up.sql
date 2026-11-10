-- 二段階認証の設定済み判定を、auth スキーマに入れない最小権限の実行ロールからも行えるようにする（ops #62）。
-- Supabase の postgres は auth スキーマの権限を他のロールへ渡せないため、postgres 所有の関数を経由する。
-- 判定は従来の直接の読み取りと同じ：auth.mfa_factors にそのユーザーの status = 'verified' の行があるか。
-- plpgsql にしているのは、auth.mfa_factors の無い素の Postgres（手元・CI）でも作成でき、呼んだときに
-- 従来と同じエラー（表が無い 42P01／スキーマが無い 3F000／権限が無い 42501）になるようにするため。
CREATE FUNCTION public.mfa_verified(target uuid) RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    RETURN EXISTS (SELECT 1 FROM auth.mfa_factors f WHERE f.user_id = target AND f.status = 'verified');
END;
$$;

REVOKE ALL ON FUNCTION public.mfa_verified(uuid) FROM PUBLIC;
-- Supabase が既定で関数に付ける API 用ロールの実行権限も外す（ロールがある環境だけ）。
DO $$
DECLARE r text;
BEGIN
    FOREACH r IN ARRAY ARRAY['anon', 'authenticated', 'service_role'] LOOP
        IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r) THEN
            EXECUTE format('REVOKE ALL ON FUNCTION public.mfa_verified(uuid) FROM %I', r);
        END IF;
    END LOOP;
    -- 公開 API の最小権限の実行ロール（本番で運営者が作成済み）にだけ実行を許す。
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'track_talk_api') THEN
        GRANT EXECUTE ON FUNCTION public.mfa_verified(uuid) TO track_talk_api;
    END IF;
END
$$;
