ALTER TABLE coaching_contracts DROP CONSTRAINT coaching_contracts_fulfillment_version_check;
ALTER TABLE coaching_contracts ADD CONSTRAINT coaching_contracts_fulfillment_version_check CHECK (fulfillment_version IN (0,1));
ALTER TABLE coaching_payments DROP CONSTRAINT coaching_payments_refund_reason_check;
ALTER TABLE coaching_payments ADD CONSTRAINT coaching_payments_refund_reason_check CHECK (refund_reason IN ('no_coach_reply', 'not_started', 'admin_full', 'admin_remaining', 'support_unprovided', 'support_admin_full', 'support_no_response'));
DROP TABLE coaching_in_person_sessions;
ALTER TABLE coaching_contracts DROP COLUMN venue_area,DROP COLUMN venue_name,DROP COLUMN session_count,DROP COLUMN session_minutes,DROP COLUMN travel_fee_yen,DROP COLUMN max_participants,DROP COLUMN delivered_session_count;
ALTER TABLE coaching_plans DROP COLUMN venue_area,DROP COLUMN venue_name,DROP COLUMN session_count,DROP COLUMN session_minutes,DROP COLUMN travel_fee_yen,DROP COLUMN max_participants;
