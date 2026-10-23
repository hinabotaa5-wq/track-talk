-- チケット購入と利用を分け、購入ロットごとに残高・返金額を追跡する。
CREATE TABLE coaching_ticket_products (
    id text PRIMARY KEY,
    title text NOT NULL,
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 100),
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    active boolean NOT NULL DEFAULT true
);
INSERT INTO coaching_ticket_products (id, title, quantity, amount_yen) VALUES
    ('single', 'チケット1枚', 1, 500), ('five', 'チケット5枚', 5, 2400);

CREATE TABLE coaching_ticket_purchases (
    id uuid PRIMARY KEY,
    owner_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    request_key uuid NOT NULL,
    product_id text NOT NULL REFERENCES coaching_ticket_products(id),
    quantity integer NOT NULL CHECK (quantity > 0),
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'partially_refunded', 'refunded')),
    checkout_session_id text UNIQUE,
    checkout_url text,
    session_expires_at timestamptz NOT NULL,
    payment_intent_id text UNIQUE,
    return_path text NOT NULL,
    paid_at timestamptz,
    refunded_yen integer NOT NULL DEFAULT 0 CHECK (refunded_yen >= 0 AND refunded_yen <= amount_yen),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(owner_id, request_key),
    CHECK (status = 'pending' OR (payment_intent_id IS NOT NULL AND paid_at IS NOT NULL))
);
CREATE INDEX coaching_ticket_purchases_owner_idx ON coaching_ticket_purchases(owner_id, created_at);

CREATE TABLE coaching_ticket_lots (
    id uuid PRIMARY KEY REFERENCES coaching_ticket_purchases(id),
    owner_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    quantity integer NOT NULL CHECK (quantity > 0),
    available integer NOT NULL CHECK (available >= 0),
    reserved integer NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    consumed integer NOT NULL DEFAULT 0 CHECK (consumed >= 0),
    -- 返金処理中の枚数も含め、すぐに利用可能残高から除く。
    refunded integer NOT NULL DEFAULT 0 CHECK (refunded >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (quantity = available + reserved + consumed + refunded)
);
CREATE INDEX coaching_ticket_lots_owner_idx ON coaching_ticket_lots(owner_id, created_at, id);

CREATE TABLE coaching_ticket_reservations (
    id uuid PRIMARY KEY,
    owner_id uuid REFERENCES profiles(id) ON DELETE SET NULL,
    request_id uuid UNIQUE REFERENCES review_requests(id) ON DELETE SET NULL,
    quantity integer NOT NULL CHECK (quantity > 0),
    -- 割引前の指導価格と報酬率は依頼時に確定する。
    nominal_yen integer NOT NULL CHECK (nominal_yen > 0),
    share_percent smallint NOT NULL CHECK (share_percent BETWEEN 1 AND 100),
    status text NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved', 'consumed', 'released')),
    created_at timestamptz NOT NULL DEFAULT now(),
    settled_at timestamptz
);
CREATE TABLE coaching_ticket_allocations (
    reservation_id uuid NOT NULL REFERENCES coaching_ticket_reservations(id),
    lot_id uuid NOT NULL REFERENCES coaching_ticket_lots(id),
    quantity integer NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (reservation_id, lot_id)
);
CREATE TABLE coaching_ticket_refunds (
    id uuid PRIMARY KEY,
    purchase_id uuid NOT NULL REFERENCES coaching_ticket_purchases(id),
    quantity integer NOT NULL CHECK (quantity > 0),
    amount_yen integer NOT NULL CHECK (amount_yen > 0),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'succeeded', 'failed')),
    stripe_refund_id text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX coaching_ticket_refunds_pending_idx ON coaching_ticket_refunds(purchase_id) WHERE status = 'pending';

CREATE TABLE coaching_ticket_ledger (
    id uuid PRIMARY KEY,
    lot_id uuid NOT NULL REFERENCES coaching_ticket_lots(id),
    operation_key text NOT NULL UNIQUE,
    kind text NOT NULL CHECK (kind IN ('purchase', 'reserve', 'consume', 'release', 'refund', 'refund_failed')),
    available_delta integer NOT NULL DEFAULT 0,
    reserved_delta integer NOT NULL DEFAULT 0,
    consumed_delta integer NOT NULL DEFAULT 0,
    refunded_delta integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX coaching_ticket_ledger_lot_idx ON coaching_ticket_ledger(lot_id, created_at);

ALTER TABLE coaching_ticket_products ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_ticket_purchases ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_ticket_lots ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_ticket_reservations ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_ticket_allocations ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_ticket_refunds ENABLE ROW LEVEL SECURITY;
ALTER TABLE coaching_ticket_ledger ENABLE ROW LEVEL SECURITY;
