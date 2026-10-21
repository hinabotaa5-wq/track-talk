-- 000077 で作った injury_group_hindsight だけ RLS が有効になっていなかった。ほかの表と同じく、
-- 公開されている anon key で PostgREST から読み書きできないようにする（API は RLS を通らない接続で使う）。
ALTER TABLE injury_group_hindsight ENABLE ROW LEVEL SECURITY;

-- これから作る表・シーケンス・関数にも、Supabase の既定で anon / authenticated に付く権限を付けない。
-- RLS の付け忘れがあっても、PostgREST から触れないようにするための二重の備え。Web は表を直接読まない。
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON TABLE injury_group_hindsight FROM anon;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM anon;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM anon;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE EXECUTE ON FUNCTIONS FROM anon;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'authenticated') THEN
        REVOKE ALL ON TABLE injury_group_hindsight FROM authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE EXECUTE ON FUNCTIONS FROM authenticated;
    END IF;
END $$;
