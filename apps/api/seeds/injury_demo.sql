-- 開発用のケガデータ。ケガ部の公開ページ・集計(予防・復帰の目安・みんなの一言)が表示される程度の人数を入れる。
-- 何度流しても同じ結果になる(デモ用プロフィールを消してから入れ直し、乱数は固定)。
-- 実行: make seed-injury(ローカルの docker の DB にだけ流す)
BEGIN;

SELECT setseed(0.42);

-- デモ用プロフィールは 00000000-0000-4000-8000-0000000000NN の形。消すとグループへの参加と段階の記録も一緒に消える。
DELETE FROM profiles WHERE id::text LIKE '00000000-0000-4000-8000-0000000000%';

DO $$
DECLARE
    -- グループごとの人数(診断名別。「その他」は自由記述の診断名つき)
    groups text[]  := ARRAY['hamstring-strain', 'sprain', 'shin-splints', 'achilles-tendinitis', 'jumpers-knee', 'quad-strain', 'shoulder', 'elbow', 'other'];
    counts int[]   := ARRAY[14, 10, 9, 7, 6, 6, 5, 4, 4];
    stages text[]  := ARRAY['resting', 'rehab', 'jogging', 'practice', 'race'];
    places text[]  := ARRAY['tartan', 'grass', 'dirt', 'asphalt', 'slope', 'stairs', 'weight-room', 'gym'];
    activities text[] := ARRAY['jog', 'start', 'sprint', 'jump-hurdle', 'throw', 'weights', 'outside'];
    events text[]  := ARRAY['sprint', 'hurdles', 'middle', 'long', 'jumps', 'throws', 'combined', 'race-walk'];
    changes text[] := ARRAY['more-volume', 'new-shoes', 'comeback', 'fatigue', 'played-through-pain'];
    all_actions text[] := ARRAY['hospital-rehab', 'osteopath', 'stretch', 'bath', 'strength', 'icing', 'taping', 'rest'];
    others text[]  := ARRAY['腸脛靭帯炎', '腰椎分離症', '足底筋膜炎', '疲労骨折'];
    notes text[]   := ARRAY[
        '痛みが引いても焦らず、ジョグは10分から始めました。',
        '病院で言われた期間を守ったのが結局いちばん早かった。',
        '再開してから1週間は、練習後のアイシングを欠かさなかった。',
        'ストレッチを毎日やるようにしたら、張りが出にくくなりました。',
        '無理に走って悪化させたので、次の人は早めに休んでほしい。',
        'ウォームアップを長めにとるようにしました。',
        '接骨院で原因のフォームの癖を指摘してもらえたのが大きかった。',
        '体幹トレを足したら、復帰後の違和感がなくなった。',
        '大会に間に合わせようとして遅れたので、目標より体を優先したほうがいいです。',
        'シューズを替えただけで痛みが変わりました。',
        '最初の1週間は何もしないで寝るのがいちばんつらかった。',
        '段階を戻したくなったけど、一度戻して整えたら安定した。'
    ];
    n int := 0;
    g int;
    i int;
    uid uuid;
    part text;
    sev text;
    mult numeric;
    final_idx int;
    s int;
    offsets numeric[];
    day_total numeric;
    t timestamptz;
    injured date;
    act text[];
    chg text[];
    r numeric;
    note text;
    started timestamptz;
BEGIN
    FOR g IN 1 .. array_length(groups, 1) LOOP
        part := groups[g];
        FOR i IN 1 .. counts[g] LOOP
            n := n + 1;
            uid := ('00000000-0000-4000-8000-' || lpad(n::text, 12, '0'))::uuid;

            INSERT INTO profiles (id, nickname, nickname_public)
            VALUES (uid, 'デモ' || n, true);

            r := random();
            sev := CASE WHEN r < 0.45 THEN 'mild' WHEN r < 0.85 THEN 'moderate' ELSE 'severe' END;
            mult := CASE sev WHEN 'mild' THEN 1 WHEN 'moderate' THEN 1.8 ELSE 3 END;

            -- いまの段階: 回復の途中にいる人と、もう戻った人を混ぜる
            r := random();
            final_idx := CASE WHEN r < 0.12 THEN 1 WHEN r < 0.32 THEN 2 WHEN r < 0.5 THEN 3 WHEN r < 0.7 THEN 4 ELSE 5 END;

            -- 各段階に最初に着くまでの日数(ケガをした日から)。後の段階ほど遅く、ばらつきを入れる
            offsets := ARRAY[
                0,
                round((3 + random() * 5) * mult),
                round((10 + random() * 10) * mult),
                round((18 + random() * 12) * mult),
                round((28 + random() * 17) * mult)
            ];
            FOR s IN 2 .. 5 LOOP
                offsets[s] := GREATEST(offsets[s], offsets[s - 1] + 1);
            END LOOP;
            day_total := offsets[final_idx];

            -- 最後の段階に着いてから0〜10日たっている
            injured := (now() - make_interval(days => (day_total + floor(random() * 11))::int))::date;
            started := injured::timestamptz + interval '10 hours';

            chg := CASE WHEN random() < 0.2 THEN ARRAY['none']
                        ELSE (SELECT array_agg(c) FROM (SELECT c FROM unnest(changes) c ORDER BY random() LIMIT 1 + floor(random() * 2)::int) x) END;

            INSERT INTO injury_group_members (
                user_id, body_part, injured_on, stage, joined_at, updated_at,
                diagnosis, severity, saw_doctor, place, activity, pre_changes, event
            ) VALUES (
                uid, part, injured, stages[final_idx], started, started,
                CASE WHEN part = 'other' THEN others[1 + floor(random() * array_length(others, 1))::int] END,
                sev, random() < 0.6,
                places[1 + floor(random() * array_length(places, 1))::int],
                activities[1 + floor(random() * array_length(activities, 1))::int],
                chg,
                events[1 + floor(random() * array_length(events, 1))::int]
            );

            -- 入った記録(最初の段階。やったことと一言はなし)
            INSERT INTO injury_group_stage_log (user_id, body_part, stage, created_at)
            VALUES (uid, part, 'resting', started);

            -- 段階を変えるたびの記録
            FOR s IN 2 .. final_idx LOOP
                t := injured::timestamptz + make_interval(days => offsets[s]::int) + interval '18 hours';
                IF t > now() THEN t := now() - interval '1 hour'; END IF;
                act := (SELECT array_agg(a) FROM (SELECT a FROM unnest(all_actions) a ORDER BY random() LIMIT 1 + floor(random() * 3)::int) x);
                IF random() < 0.1 THEN act := ARRAY['nothing']; END IF;
                note := NULL;
                IF random() < 0.4 THEN note := notes[1 + floor(random() * array_length(notes, 1))::int]; END IF;
                INSERT INTO injury_group_stage_log (user_id, body_part, stage, created_at, actions, note, note_public)
                VALUES (uid, part, stages[s], t, act, note, note IS NOT NULL AND random() < 0.75);
            END LOOP;
        END LOOP;
    END LOOP;
END $$;

COMMIT;
