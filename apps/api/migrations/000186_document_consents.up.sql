CREATE TABLE document_consents (
    id bigserial PRIMARY KEY,
    user_id uuid NOT NULL,
    document text NOT NULL CHECK (document IN ('terms', 'guidelines', 'privacy')),
    version text NOT NULL,
    consented_at timestamptz NOT NULL DEFAULT now(),
    account_deleted_at timestamptz,
    last_transaction_at timestamptz,
    UNIQUE (user_id, document, version)
);
CREATE INDEX document_consents_user_id_idx ON document_consents (user_id);
ALTER TABLE document_consents ENABLE ROW LEVEL SECURITY;
