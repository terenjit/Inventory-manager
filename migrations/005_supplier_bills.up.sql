-- Simple supplier bill tracking (nota supplier).
-- Stores a free-text supplier name and a single total amount; no line items.
-- This mirrors the front-end SupplierBill shape exactly.
CREATE TABLE supplier_bills (
  id            BIGINT UNSIGNED  AUTO_INCREMENT PRIMARY KEY,
  supplier_name VARCHAR(150)     NOT NULL,
  invoice_no    VARCHAR(100)     NOT NULL,
  invoice_date  DATE             NOT NULL,
  received_date DATE             NULL,
  amount        DECIMAL(15,2)    NOT NULL DEFAULT 0,
  due_date      DATE             NULL,
  status        ENUM('unpaid','paid') NOT NULL DEFAULT 'unpaid',
  created_by    INT UNSIGNED     NOT NULL,
  deleted_at    DATETIME(3)      NULL,
  created_at    DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  CONSTRAINT fk_sbill_user FOREIGN KEY (created_by) REFERENCES users (id),
  CONSTRAINT chk_sbill_amount CHECK (amount >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
