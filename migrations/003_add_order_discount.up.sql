-- Add discount columns to orders.
-- Only one of discount_pct or discount_rp may be non-zero per order.
-- discount_amount stores the computed Rp value that was actually deducted.
-- total formula changes to: subtotal + tax - discount_amount

ALTER TABLE orders
  DROP CHECK chk_orders_total,
  ADD COLUMN discount_pct    DECIMAL(5,2)  NOT NULL DEFAULT 0 AFTER total,
  ADD COLUMN discount_rp     DECIMAL(15,2) NOT NULL DEFAULT 0 AFTER discount_pct,
  ADD COLUMN discount_amount DECIMAL(15,2) NOT NULL DEFAULT 0 AFTER discount_rp,
  ADD CONSTRAINT chk_orders_total    CHECK (total = subtotal + tax - discount_amount),
  ADD CONSTRAINT chk_orders_discount CHECK (discount_pct = 0 OR discount_rp = 0);
