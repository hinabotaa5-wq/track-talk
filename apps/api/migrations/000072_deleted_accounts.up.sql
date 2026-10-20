-- 退会したアカウントの ID。退会のあとも手元に残ったアクセストークンが期限切れになるまで使えるので、
-- そのトークンでプロフィールが作り直されないように、ここにある ID ではプロフィールを作らない。
CREATE TABLE deleted_accounts (
    id uuid PRIMARY KEY,
    deleted_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE deleted_accounts ENABLE ROW LEVEL SECURITY;
