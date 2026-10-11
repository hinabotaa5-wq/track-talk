CREATE TABLE inquiries (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    category text NOT NULL CHECK (category IN ('bug', 'account', 'request', 'other')),
    subject varchar(80) NOT NULL,
    body text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX inquiries_user_created_idx ON inquiries (user_id, created_at DESC);
CREATE INDEX inquiries_created_idx ON inquiries (created_at DESC);

ALTER TABLE inquiries ENABLE ROW LEVEL SECURITY;
