ALTER TABLE minor_purchase_declarations
    DROP COLUMN declaration_text,
    DROP COLUMN notice_text,
    DROP COLUMN amount_yen,
    DROP COLUMN coach_id,
    DROP COLUMN declared_birth_month,
    DROP COLUMN client_shown_at,
    DROP COLUMN client_checked_at;
