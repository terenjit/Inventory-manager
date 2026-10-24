ALTER TABLE supplier_invoices
  DROP CHECK chk_sinv_amount,
  MODIFY COLUMN status ENUM('unpaid','paid') NOT NULL DEFAULT 'unpaid',
  DROP COLUMN paid_amount,
  ADD CONSTRAINT chk_sinv_amount CHECK (amount >= 0);
