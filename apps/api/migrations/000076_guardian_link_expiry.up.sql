-- 保護者のリンクは、出してから30日で使えなくする。既にあるリンクは、この日から数える。
ALTER TABLE guardian_consents
    ADD COLUMN link_issued_at timestamptz NOT NULL DEFAULT now();
