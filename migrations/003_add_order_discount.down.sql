ALTER TABLE orders
  DROP CHECK chk_orders_discount,
  DROP CHECK chk_orders_total,
  DROP COLUMN discount_pct,
  DROP COLUMN discount_rp,
  DROP COLUMN discount_amount,
  ADD CONSTRAINT chk_orders_total CHECK (total = subtotal + tax);
