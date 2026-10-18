-- 保護者の同意のリンクは、本人に渡さず API が保護者のメールあてに送る（本人だけで同意を済ませられないようにする）。
-- guardian_email は送り先。保護者が一度答えたあとは本人からは変えられない。
-- link_sent_on と link_sends は、1日（日本時間）に送れる回数を数える。
ALTER TABLE guardian_consents
    ADD COLUMN guardian_email text,
    ADD COLUMN link_sent_on date,
    ADD COLUMN link_sends integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT guardian_consents_guardian_email_check CHECK (guardian_email IS NULL OR char_length(guardian_email) BETWEEN 3 AND 254);
