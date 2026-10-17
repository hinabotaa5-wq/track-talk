DROP TABLE coaching_payments;

UPDATE coaching_contracts SET status = 'ended' WHERE status = 'refunded';

ALTER TABLE coaching_contracts
    DROP CONSTRAINT coaching_contracts_period_check,
    ADD CONSTRAINT coaching_contracts_period_check CHECK (
        status NOT IN ('active', 'suspended', 'ended') OR (starts_at IS NOT NULL AND ends_at IS NOT NULL)
    ),
    DROP CONSTRAINT coaching_contracts_status_check,
    ADD CONSTRAINT coaching_contracts_status_check CHECK (status IN (
        'applied', 'accepted', 'active', 'suspended', 'ended', 'declined', 'guardian_declined', 'expired', 'cancelled'
    )),
    DROP COLUMN payment_required;
