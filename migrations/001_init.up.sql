-- Textile POS — initial schema
-- Requires MySQL 8.0.16+ (CHECK constraints are enforced from 8.0.16).
-- Create the database with: CREATE DATABASE textile_pos CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- All DATETIME columns store UTC. Convert to WIB (UTC+7) in the app.
-- Quantities are in meters, DECIMAL(12,2). Money is DECIMAL(15,2), in Rupiah.

-- ─────────────────────────── Master data ───────────────────────────

CREATE TABLE stores (
  id          INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  code        VARCHAR(10)  NOT NULL,                 -- short code used in document numbers, e.g. TU
  name        VARCHAR(100) NOT NULL,                 -- e.g. Toko Utama
  is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_stores_code (code)
) ENGINE=InnoDB;

CREATE TABLE users (
  id             INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  username       VARCHAR(50)  NOT NULL,
  full_name      VARCHAR(100) NOT NULL,
  password_hash  VARCHAR(255) NOT NULL,              -- bcrypt / argon2id
  role           ENUM('admin','cashier') NOT NULL,
  store_id       INT UNSIGNED NULL,                  -- required for cashier, NULL for admin
  is_active      BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_users_username (username),
  CONSTRAINT fk_users_store FOREIGN KEY (store_id) REFERENCES stores (id),
  CONSTRAINT chk_users_cashier_store CHECK (role = 'admin' OR store_id IS NOT NULL)
) ENGINE=InnoDB;

CREATE TABLE categories (
  id    INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name  VARCHAR(80) NOT NULL,
  UNIQUE KEY uq_categories_name (name)
) ENGINE=InnoDB;

CREATE TABLE suppliers (
  id          INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name        VARCHAR(120) NOT NULL,
  phone       VARCHAR(30)  NULL,
  address     VARCHAR(255) NULL,
  note        VARCHAR(255) NULL,
  is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_suppliers_name (name)
) ENGINE=InnoDB;

CREATE TABLE salespeople (                           -- "Daftar Sales"
  id          INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name        VARCHAR(100) NOT NULL,
  is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB;

CREATE TABLE products (                              -- "Daftar Item"
  id           INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  code         VARCHAR(50)  NOT NULL,                -- store UPPERCASE; collation is case-insensitive
  name         VARCHAR(150) NOT NULL,
  category_id  INT UNSIGNED NULL,
  supplier_id  INT UNSIGNED NULL,
  color        VARCHAR(50)  NULL,
  material     VARCHAR(80)  NULL,
  width_cm     SMALLINT UNSIGNED NULL,
  sell_price   DECIMAL(15,2) NOT NULL,               -- harga jual per meter
  cost_price   DECIMAL(15,2) NOT NULL DEFAULT 0,     -- harga modal per meter (admin only)
  min_stock_m  DECIMAL(12,2) NOT NULL DEFAULT 0,     -- low-stock threshold, applied per store
  is_active    BOOLEAN       NOT NULL DEFAULT TRUE,  -- "Hapus" = soft delete
  created_at   DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at   DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_products_code (code),
  KEY idx_products_name (name),
  CONSTRAINT fk_products_category FOREIGN KEY (category_id) REFERENCES categories (id),
  CONSTRAINT fk_products_supplier FOREIGN KEY (supplier_id) REFERENCES suppliers (id),
  CONSTRAINT chk_products_prices CHECK (sell_price >= 0 AND cost_price >= 0 AND min_stock_m >= 0)
) ENGINE=InnoDB;

-- ─────────────────────────── Stock ───────────────────────────

CREATE TABLE inventory (                             -- current stock per store
  store_id    INT UNSIGNED NOT NULL,
  product_id  INT UNSIGNED NOT NULL,
  qty_m       DECIMAL(12,2) NOT NULL DEFAULT 0,
  updated_at  DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (store_id, product_id),
  CONSTRAINT fk_inventory_store   FOREIGN KEY (store_id)   REFERENCES stores (id),
  CONSTRAINT fk_inventory_product FOREIGN KEY (product_id) REFERENCES products (id),
  CONSTRAINT chk_inventory_qty CHECK (qty_m >= 0)    -- last line of defense against overselling
) ENGINE=InnoDB;

-- Per-store, per-day document counters: INV/TU/20261006/0001
CREATE TABLE doc_sequences (
  store_id  INT UNSIGNED NOT NULL,
  doc_type  ENUM('order','transfer','opname','purchase') NOT NULL,
  period    DATE NOT NULL,
  last_no   INT UNSIGNED NOT NULL,
  PRIMARY KEY (store_id, doc_type, period)
) ENGINE=InnoDB;

-- ─────────────────────────── Sales (Kasir) ───────────────────────────

CREATE TABLE orders (
  id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  order_no       VARCHAR(40)   NOT NULL,
  store_id       INT UNSIGNED  NOT NULL,
  cashier_id     INT UNSIGNED  NOT NULL,             -- "User Buat"
  sales_id       INT UNSIGNED  NULL,
  customer_name  VARCHAR(100)  NULL,
  note           VARCHAR(255)  NULL,
  subtotal       DECIMAL(15,2) NOT NULL,
  tax            DECIMAL(15,2) NOT NULL DEFAULT 0,
  total          DECIMAL(15,2) NOT NULL,
  paid_amount    DECIMAL(15,2) NOT NULL DEFAULT 0,   -- sum of payments; "Sisa" = total - paid_amount
  status         ENUM('unpaid','partial','paid','void') NOT NULL,
  updated_by     INT UNSIGNED  NULL,                 -- "User Ubah"
  void_reason    VARCHAR(255)  NULL,
  voided_by      INT UNSIGNED  NULL,
  voided_at      DATETIME(3)   NULL,
  created_at     DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_orders_no (order_no),
  KEY idx_orders_store_created (store_id, created_at),
  KEY idx_orders_status (status),
  CONSTRAINT fk_orders_store   FOREIGN KEY (store_id)   REFERENCES stores (id),
  CONSTRAINT fk_orders_cashier FOREIGN KEY (cashier_id) REFERENCES users (id),
  CONSTRAINT fk_orders_sales   FOREIGN KEY (sales_id)   REFERENCES salespeople (id),
  CONSTRAINT fk_orders_updby   FOREIGN KEY (updated_by) REFERENCES users (id),
  CONSTRAINT fk_orders_voidby  FOREIGN KEY (voided_by)  REFERENCES users (id),
  CONSTRAINT chk_orders_total  CHECK (total = subtotal + tax),
  CONSTRAINT chk_orders_paid   CHECK (paid_amount >= 0 AND paid_amount <= total)
) ENGINE=InnoDB;

CREATE TABLE order_items (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  order_id    BIGINT UNSIGNED NOT NULL,
  product_id  INT UNSIGNED    NOT NULL,
  qty_m       DECIMAL(12,2)   NOT NULL,
  unit_price  DECIMAL(15,2)   NOT NULL,              -- snapshot of sell_price at sale time
  unit_cost   DECIMAL(15,2)   NOT NULL,              -- snapshot of cost_price, for margin reports
  line_total  DECIMAL(15,2)   NOT NULL,
  UNIQUE KEY uq_order_items_product (order_id, product_id),  -- app merges duplicate lines
  CONSTRAINT fk_order_items_order   FOREIGN KEY (order_id)   REFERENCES orders (id),
  CONSTRAINT fk_order_items_product FOREIGN KEY (product_id) REFERENCES products (id),
  CONSTRAINT chk_order_items_qty CHECK (qty_m > 0)
) ENGINE=InnoDB;

CREATE TABLE payments (                              -- one order, many payments (partial / open bills)
  id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  order_id     BIGINT UNSIGNED NOT NULL,
  method       ENUM('QRIS','TRANSFER','CASH') NOT NULL,
  amount       DECIMAL(15,2)   NOT NULL,             -- amount applied to the bill (excludes cash change)
  reference    VARCHAR(100)    NULL,                 -- transfer / QRIS reference
  received_by  INT UNSIGNED    NOT NULL,
  created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_payments_order (order_id),
  KEY idx_payments_method_created (method, created_at),
  CONSTRAINT fk_payments_order FOREIGN KEY (order_id)    REFERENCES orders (id),
  CONSTRAINT fk_payments_user  FOREIGN KEY (received_by) REFERENCES users (id),
  CONSTRAINT chk_payments_amount CHECK (amount > 0)
) ENGINE=InnoDB;

-- ─────────────────────────── Transfers between stores ───────────────────────────
-- Barang Keluar creates a transfer (status=sent, source stock decremented).
-- Barang Masuk confirms it (status=received, destination stock incremented).

CREATE TABLE stock_transfers (
  id             BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  transfer_no    VARCHAR(40)  NOT NULL,
  from_store_id  INT UNSIGNED NOT NULL,
  to_store_id    INT UNSIGNED NOT NULL,
  status         ENUM('sent','received','cancelled') NOT NULL DEFAULT 'sent',
  note           VARCHAR(255) NULL,
  sent_by        INT UNSIGNED NOT NULL,
  sent_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  received_by    INT UNSIGNED NULL,
  received_at    DATETIME(3)  NULL,
  UNIQUE KEY uq_transfers_no (transfer_no),
  KEY idx_transfers_to_status (to_store_id, status),
  KEY idx_transfers_from_sent (from_store_id, sent_at),
  CONSTRAINT fk_transfers_from    FOREIGN KEY (from_store_id) REFERENCES stores (id),
  CONSTRAINT fk_transfers_to      FOREIGN KEY (to_store_id)   REFERENCES stores (id),
  CONSTRAINT fk_transfers_sentby  FOREIGN KEY (sent_by)       REFERENCES users (id),
  CONSTRAINT fk_transfers_recvby  FOREIGN KEY (received_by)   REFERENCES users (id),
  CONSTRAINT chk_transfers_stores CHECK (from_store_id <> to_store_id)
) ENGINE=InnoDB;

CREATE TABLE stock_transfer_items (
  transfer_id  BIGINT UNSIGNED NOT NULL,
  product_id   INT UNSIGNED    NOT NULL,
  qty_m        DECIMAL(12,2)   NOT NULL,
  PRIMARY KEY (transfer_id, product_id),
  CONSTRAINT fk_transfer_items_transfer FOREIGN KEY (transfer_id) REFERENCES stock_transfers (id),
  CONSTRAINT fk_transfer_items_product  FOREIGN KEY (product_id)  REFERENCES products (id),
  CONSTRAINT chk_transfer_items_qty CHECK (qty_m > 0)
) ENGINE=InnoDB;

-- ─────────────────────────── Supplier purchases ───────────────────────────
-- "Daftar Supplier" nota. Receiving the goods adds stock to store_id.

CREATE TABLE supplier_invoices (
  id                   BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  supplier_id          INT UNSIGNED  NOT NULL,
  store_id             INT UNSIGNED  NOT NULL,       -- where the goods arrive
  invoice_no           VARCHAR(60)   NOT NULL,       -- supplier's nota number
  invoice_date         DATE          NOT NULL,
  goods_received_date  DATE          NULL,
  amount               DECIMAL(15,2) NOT NULL,
  due_date             DATE          NULL,           -- jatuh tempo
  status               ENUM('unpaid','paid') NOT NULL DEFAULT 'unpaid',
  paid_at              DATETIME(3)   NULL,
  note                 VARCHAR(255)  NULL,
  created_by           INT UNSIGNED  NOT NULL,
  created_at           DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uq_supplier_invoice (supplier_id, invoice_no),
  KEY idx_supplier_invoices_status_due (status, due_date),
  CONSTRAINT fk_sinv_supplier FOREIGN KEY (supplier_id) REFERENCES suppliers (id),
  CONSTRAINT fk_sinv_store    FOREIGN KEY (store_id)    REFERENCES stores (id),
  CONSTRAINT fk_sinv_user     FOREIGN KEY (created_by)  REFERENCES users (id),
  CONSTRAINT chk_sinv_amount CHECK (amount >= 0)
) ENGINE=InnoDB;

CREATE TABLE supplier_invoice_items (
  invoice_id  BIGINT UNSIGNED NOT NULL,
  product_id  INT UNSIGNED    NOT NULL,
  qty_m       DECIMAL(12,2)   NOT NULL,
  unit_cost   DECIMAL(15,2)   NOT NULL,
  PRIMARY KEY (invoice_id, product_id),
  CONSTRAINT fk_sinv_items_invoice FOREIGN KEY (invoice_id) REFERENCES supplier_invoices (id),
  CONSTRAINT fk_sinv_items_product FOREIGN KEY (product_id) REFERENCES products (id),
  CONSTRAINT chk_sinv_items_qty CHECK (qty_m > 0 AND unit_cost >= 0)
) ENGINE=InnoDB;

-- ─────────────────────────── Stock opname ───────────────────────────
-- On finalize, apply delta = counted_qty_m - system_qty_m to the CURRENT stock
-- (not overwrite), so sales made during the count are not lost.

CREATE TABLE stock_opnames (
  id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  opname_no     VARCHAR(40)  NOT NULL,
  store_id      INT UNSIGNED NOT NULL,
  opname_date   DATE         NOT NULL,
  status        ENUM('draft','finalized','cancelled') NOT NULL DEFAULT 'draft',
  note          VARCHAR(255) NULL,
  started_by    INT UNSIGNED NOT NULL,
  started_at    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  finalized_by  INT UNSIGNED NULL,
  finalized_at  DATETIME(3)  NULL,
  -- MySQL has no partial unique index; this generated column allows only one draft per store.
  draft_store_id INT UNSIGNED AS (IF(status = 'draft', store_id, NULL)) STORED,
  UNIQUE KEY uq_opnames_no (opname_no),
  UNIQUE KEY uq_opnames_one_draft (draft_store_id),
  KEY idx_opnames_store_date (store_id, opname_date),
  CONSTRAINT fk_opnames_store   FOREIGN KEY (store_id)     REFERENCES stores (id),
  CONSTRAINT fk_opnames_started FOREIGN KEY (started_by)   REFERENCES users (id),
  CONSTRAINT fk_opnames_final   FOREIGN KEY (finalized_by) REFERENCES users (id)
) ENGINE=InnoDB;

CREATE TABLE stock_opname_lines (
  opname_id      BIGINT UNSIGNED NOT NULL,
  product_id     INT UNSIGNED    NOT NULL,
  system_qty_m   DECIMAL(12,2)   NOT NULL,           -- snapshot when the line was added
  counted_qty_m  DECIMAL(12,2)   NULL,               -- NULL = not counted yet
  note           VARCHAR(255)    NULL,
  PRIMARY KEY (opname_id, product_id),
  CONSTRAINT fk_opname_lines_opname  FOREIGN KEY (opname_id)  REFERENCES stock_opnames (id),
  CONSTRAINT fk_opname_lines_product FOREIGN KEY (product_id) REFERENCES products (id),
  CONSTRAINT chk_opname_lines_counted CHECK (counted_qty_m IS NULL OR counted_qty_m >= 0)
) ENGINE=InnoDB;

-- ─────────────────────────── Audit ledger ───────────────────────────
-- Every change to inventory.qty_m writes exactly one row here, in the same transaction.
-- This is the "kartu stok": SUM(delta_m) per store/product must equal inventory.qty_m.

CREATE TABLE stock_movements (
  id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  store_id     INT UNSIGNED    NOT NULL,
  product_id   INT UNSIGNED    NOT NULL,
  delta_m      DECIMAL(12,2)   NOT NULL,             -- negative = stock out
  qty_after_m  DECIMAL(12,2)   NOT NULL,
  reason       ENUM('sale','sale_void','transfer_out','transfer_in','transfer_cancel',
                    'purchase','opname','import','remnant','damaged','adjustment') NOT NULL,
  ref_type     ENUM('order','transfer','purchase','opname','manual') NULL,
  ref_id       BIGINT UNSIGNED NULL,
  note         VARCHAR(255)    NULL,
  created_by   INT UNSIGNED    NOT NULL,
  created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_movements_store_product_time (store_id, product_id, created_at),
  KEY idx_movements_ref (ref_type, ref_id),
  CONSTRAINT fk_movements_store   FOREIGN KEY (store_id)   REFERENCES stores (id),
  CONSTRAINT fk_movements_product FOREIGN KEY (product_id) REFERENCES products (id),
  CONSTRAINT fk_movements_user    FOREIGN KEY (created_by) REFERENCES users (id),
  CONSTRAINT chk_movements_delta CHECK (delta_m <> 0)
) ENGINE=InnoDB;
