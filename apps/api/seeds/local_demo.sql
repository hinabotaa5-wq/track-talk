-- ローカル確認専用のサンプル投稿。docker compose の seed サービスが、アプリ用 DB にだけ流す。
-- 本番 DB には流さない(動画は手元のファイル apps/web/public/samples/ を指すため、本番では再生できない)。
-- 何度流しても同じ結果になる(決まった ID のプロフィールと投稿を入れ直す)。
-- 投稿者の ID は、認証サービス(localauth)が作るデモユーザー
-- (LOCALAUTH_SEED_USER_ID = 00000000-0000-4000-8000-000000000001)と合わせる。
BEGIN;

DELETE FROM posts WHERE id IN (
    '00000000-0000-4000-8000-000000000101',
    '00000000-0000-4000-8000-000000000102'
);
DELETE FROM profiles WHERE id = '00000000-0000-4000-8000-000000000001';

INSERT INTO profiles (id, nickname, nickname_public, event)
VALUES ('00000000-0000-4000-8000-000000000001', 'サンプル走者', true, '100m');

-- 縦動画(720x1280)。cloudinary_public_id の local/vertical が
-- apps/web/public/samples/vertical.mp4 に対応する(API の LOCAL_MEDIA_BASE_URL)。
INSERT INTO posts (id, author_id, title, body, kind, cloudinary_public_id, created_at, updated_at)
VALUES (
    '00000000-0000-4000-8000-000000000101',
    '00000000-0000-4000-8000-000000000001',
    '【サンプル】縦動画(9:16)',
    'ローカル確認用の縦動画です。詳細画面で縦いっぱいに表示され、そのまま再生できることを確認してください。',
    'video',
    'local/vertical',
    now() - interval '1 hour',
    now() - interval '1 hour'
);

-- 横動画(1280x720)。
INSERT INTO posts (id, author_id, title, body, kind, cloudinary_public_id, created_at, updated_at)
VALUES (
    '00000000-0000-4000-8000-000000000102',
    '00000000-0000-4000-8000-000000000001',
    '【サンプル】横動画(16:9)',
    'ローカル確認用の横動画です。詳細画面で横長に表示され、そのまま再生できることを確認してください。',
    'video',
    'local/horizontal',
    now() - interval '30 minutes',
    now() - interval '30 minutes'
);

COMMIT;
