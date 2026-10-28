-- name: CoachingReconciliation :one
-- 一つのスナップショットで確認し、回答・決済の同時処理による見かけの不一致を避ける。
SELECT
 (SELECT count(*)::int FROM coaching_ticket_lots l WHERE
   l.available <> COALESCE((SELECT sum(available_delta) FROM coaching_ticket_ledger g WHERE g.lot_id=l.id),0)
   OR l.reserved <> COALESCE((SELECT sum(reserved_delta) FROM coaching_ticket_ledger g WHERE g.lot_id=l.id),0)
   OR l.consumed <> COALESCE((SELECT sum(consumed_delta) FROM coaching_ticket_ledger g WHERE g.lot_id=l.id),0)
   OR l.refunded <> COALESCE((SELECT sum(refunded_delta) FROM coaching_ticket_ledger g WHERE g.lot_id=l.id),0)
 ) AS ticket_lot_ledger_mismatches,
 (SELECT count(*)::int FROM coaching_ticket_purchases p
   WHERE p.status IN ('paid','partially_refunded','refunded')
   AND NOT EXISTS(SELECT 1 FROM coaching_ticket_lots l WHERE l.id=p.id)
 ) AS paid_purchases_without_lot,
 (SELECT count(*)::int FROM coaching_ticket_reservations t
   WHERE t.status='consumed' AND (
     (t.request_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM coach_earnings e
       WHERE e.review_request_id=t.request_id
       AND e.gross_yen=t.nominal_yen AND e.share_percent=t.share_percent
       AND e.amount_yen=t.nominal_yen::bigint*t.share_percent/100))
     OR (t.consultation_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM coach_earnings e WHERE e.consultation_id=t.consultation_id
       AND e.gross_yen=t.nominal_yen AND e.share_percent=t.share_percent
       AND e.amount_yen=t.nominal_yen::bigint*t.share_percent/100))
   )
 ) AS consumed_ticket_earnings_mismatches,
 (SELECT count(*)::int FROM coaching_contracts
   WHERE fulfillment_version IN (1,2) AND fulfillment_state IN ('refund_pending','refund_failed')
 ) AS pending_support_refunds,
 (SELECT count(*)::int FROM coaching_ticket_refunds WHERE status='pending') AS pending_ticket_refunds,
 (SELECT count(*)::int FROM coaching_consultations WHERE payment_state='refund_pending') AS pending_consultation_refunds,
 (SELECT count(*)::int FROM coach_earnings WHERE payout_id IS NULL) AS unpaid_coach_earnings;
