-- 日付・操作名・件数だけを残す。削除や退会後も件数は減らさない。
-- 過去の削除済みデータは復元せず、導入後に成功したINSERTだけを数える。
CREATE TABLE daily_usage_counts (
    day date NOT NULL,
    metric text NOT NULL CHECK (metric IN (
        'signup', 'post', 'comment', 'like', 'sense_note',
        'race_record', 'product_review', 'deletion'
    )),
    count bigint NOT NULL CHECK (count > 0),
    PRIMARY KEY (day, metric)
);
ALTER TABLE daily_usage_counts ENABLE ROW LEVEL SECURITY;

CREATE TABLE daily_usage_collection (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    started_at timestamptz NOT NULL DEFAULT statement_timestamp()
);
ALTER TABLE daily_usage_collection ENABLE ROW LEVEL SECURITY;
INSERT INTO daily_usage_collection (singleton) VALUES (true);

CREATE FUNCTION count_daily_usage() RETURNS trigger
LANGUAGE plpgsql SET search_path = public, pg_temp AS $$
BEGIN
    INSERT INTO daily_usage_counts (day, metric, count)
    VALUES ((statement_timestamp() AT TIME ZONE 'Asia/Tokyo')::date, TG_ARGV[0], 1)
    ON CONFLICT (day, metric) DO UPDATE
        SET count = daily_usage_counts.count + 1;
    RETURN NEW;
END;
$$;

CREATE TRIGGER count_daily_signup AFTER INSERT ON profiles
    FOR EACH ROW EXECUTE FUNCTION count_daily_usage('signup');
CREATE TRIGGER count_daily_post AFTER INSERT ON posts
    FOR EACH ROW WHEN (NEW.kind <> 'meet') EXECUTE FUNCTION count_daily_usage('post');
CREATE TRIGGER count_daily_comment AFTER INSERT ON comments
    FOR EACH ROW EXECUTE FUNCTION count_daily_usage('comment');
CREATE TRIGGER count_daily_like AFTER INSERT ON post_likes
    FOR EACH ROW EXECUTE FUNCTION count_daily_usage('like');
CREATE TRIGGER count_daily_sense_note AFTER INSERT ON sense_notes
    FOR EACH ROW EXECUTE FUNCTION count_daily_usage('sense_note');
CREATE TRIGGER count_daily_race_record AFTER INSERT ON race_records
    FOR EACH ROW EXECUTE FUNCTION count_daily_usage('race_record');
CREATE TRIGGER count_daily_product_review AFTER INSERT ON product_reviews
    FOR EACH ROW EXECUTE FUNCTION count_daily_usage('product_review');
CREATE TRIGGER count_daily_deletion AFTER INSERT ON deleted_accounts
    FOR EACH ROW EXECUTE FUNCTION count_daily_usage('deletion');
