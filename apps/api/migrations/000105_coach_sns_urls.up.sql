-- コーチが任意で載せる、チームや個人の SNS の URL。最大3件。連絡先そのものではなく公開ページへのリンクだけを想定する。
ALTER TABLE coach_profiles ADD COLUMN sns_urls text[] NOT NULL DEFAULT '{}' CHECK (cardinality(sns_urls) <= 3);
