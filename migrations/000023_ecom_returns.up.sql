-- Track when an order was delivered (needed for 7-day return window)
ALTER TABLE ecom_orders
    ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;

-- Update payment_status check values to include new statuses
-- UNPAID, PAID, REFUND_INITIATED, REFUNDED, PAYOUT_INITIATED, PAYOUT_COMPLETED

-- Returns table
CREATE TABLE IF NOT EXISTS ecom_returns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES ecom_orders(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES ecom_customers(id),
    reason TEXT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'REQUESTED',
    -- REQUESTED, APPROVED, REJECTED, REFUNDED, PAYOUT_INITIATED, PAYOUT_COMPLETED

    -- For COD payout
    payout_method VARCHAR(10),   -- UPI or BANK
    payout_upi VARCHAR(100),
    payout_account_number VARCHAR(50),
    payout_ifsc VARCHAR(15),
    payout_account_name VARCHAR(200),
    payout_transfer_id VARCHAR(100),

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ecom_returns_order ON ecom_returns(order_id);
CREATE INDEX idx_ecom_returns_customer ON ecom_returns(customer_id);
