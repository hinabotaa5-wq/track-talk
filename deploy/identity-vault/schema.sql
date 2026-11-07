-- Apply to the separate identity database as its migration owner.
CREATE SCHEMA identity_vault;
REVOKE ALL ON SCHEMA identity_vault FROM PUBLIC;

-- Provision with a verified identity from the dedicated issuer. The application
-- can look up these assignments but cannot grant itself a role.
CREATE TABLE identity_vault.members (
    subject uuid PRIMARY KEY,
    coach_id uuid UNIQUE,
    is_admin boolean NOT NULL DEFAULT false,
    CHECK (is_admin = (coach_id IS NULL))
);

CREATE TABLE identity_vault.identities (
    coach_id uuid PRIMARY KEY,
    key_id text NOT NULL,
    revision uuid NOT NULL DEFAULT gen_random_uuid(),
    legal_name_enc bytea NOT NULL,
    address_enc bytea NOT NULL,
    ended_at timestamptz,
    last_transaction_at timestamptz
);

CREATE TABLE identity_vault.commerce (
    coach_id uuid PRIMARY KEY,
    required boolean NOT NULL DEFAULT true,
    forced boolean NOT NULL DEFAULT false,
    exempt_declared_at timestamptz
);

-- Holds only authorizations to publish specific versions of a plan. Phone
-- belongs to the public disclosure and is never stored with the identity.
CREATE TABLE identity_vault.disclosures (
    plan_id uuid PRIMARY KEY,
    coach_id uuid NOT NULL REFERENCES identity_vault.identities ON DELETE CASCADE,
    plan_epoch uuid NOT NULL,
    coach_epoch uuid NOT NULL,
    phone text,
    CHECK (phone IS NULL OR length(phone) BETWEEN 10 AND 20)
);
CREATE INDEX identity_disclosures_coach ON identity_vault.disclosures (coach_id);

CREATE TABLE identity_vault.access_log (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    coach_id uuid NOT NULL,
    viewer uuid NOT NULL,
    purpose text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX identity_access_coach ON identity_vault.access_log (coach_id, created_at DESC);

ALTER TABLE identity_vault.members ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_vault.identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_vault.commerce ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_vault.disclosures ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity_vault.access_log ENABLE ROW LEVEL SECURITY;
