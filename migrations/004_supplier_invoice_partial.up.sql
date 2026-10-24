-- Add partial payment status and paid_amount tracking to supplier_invoices.
-- Status is auto-computed from paid_amount in the service layer:
--   paid_amount = 0            → unpaid
--   0 < paid_amount < amount   → partial
--   paid_amount >= amount      → paid

ALTER TABLE supplier_invoices
  DROP CHECK chk_sinv_amount,
  MODIFY COLUMN status ENUM('unpaid','partial','paid') NOT NULL DEFAULT 'unpaid',
  ADD COLUMN paid_amount DECIMAL(15,2) NOT NULL DEFAULT 0 AFTER amount,
  ADD CONSTRAINT chk_sinv_amount CHECK (amount >= 0 AND paid_amount >= 0 AND paid_amount <= amount);
