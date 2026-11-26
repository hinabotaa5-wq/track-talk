-- 注意：この down は、登録された生まれた年（profiles.birth_year）を消す。消した値は戻せない。
ALTER TABLE profiles
    DROP CONSTRAINT profiles_birth_year_range,
    DROP COLUMN birth_year;
