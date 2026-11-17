ALTER TABLE coaching_messages DROP COLUMN system_kind;
DROP TABLE coaching_absence_extensions;
ALTER TABLE coaching_contracts
    DROP CONSTRAINT coaching_contracts_absence_check,
    DROP COLUMN absence_days,
    DROP COLUMN absence_return_on,
    DROP COLUMN absence_from;
