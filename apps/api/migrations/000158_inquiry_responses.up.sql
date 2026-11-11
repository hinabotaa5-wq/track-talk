-- 開示請求への応答の記録（#357 項目5）。請求（created_at）と応答（responded_at）の
-- 日時を証跡として残す。応答の内容と応答者もあわせて記録する。
ALTER TABLE inquiries
    ADD COLUMN responded_at timestamptz,
    ADD COLUMN response_body text,
    ADD COLUMN responder_id uuid REFERENCES profiles (id) ON DELETE SET NULL;

CREATE INDEX inquiries_responded_idx ON inquiries (responded_at DESC) WHERE responded_at IS NOT NULL;
