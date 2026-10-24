-- Remove textile-specific columns, add unit, rename qty columns to be unit-agnostic,
-- and add commission_rate to salespeople.

-- ── products ──────────────────────────────────────────────────────────────────
ALTER TABLE products
  DROP CHECK chk_products_prices,
  DROP COLUMN color,
  DROP COLUMN material,
  DROP COLUMN width_cm,
  ADD COLUMN unit ENUM('pcs','meter') NOT NULL DEFAULT 'pcs' AFTER supplier_id,
  RENAME COLUMN min_stock_m TO min_stock,
  ADD CONSTRAINT chk_products_prices CHECK (sell_price >= 0 AND cost_price >= 0 AND min_stock >= 0);

-- ── salespeople ───────────────────────────────────────────────────────────────
ALTER TABLE salespeople
  ADD COLUMN commission_rate DECIMAL(5,2) NOT NULL DEFAULT 0
    COMMENT 'Agreed commission percentage, e.g. 5.00 = 5%';

-- ── inventory ─────────────────────────────────────────────────────────────────
ALTER TABLE inventory
  DROP CHECK chk_inventory_qty,
  RENAME COLUMN qty_m TO qty,
  ADD CONSTRAINT chk_inventory_qty CHECK (qty >= 0);

-- ── order_items ───────────────────────────────────────────────────────────────
ALTER TABLE order_items
  DROP CHECK chk_order_items_qty,
  RENAME COLUMN qty_m TO qty,
  ADD CONSTRAINT chk_order_items_qty CHECK (qty > 0);

-- ── stock_movements ───────────────────────────────────────────────────────────
ALTER TABLE stock_movements
  DROP CHECK chk_movements_delta,
  RENAME COLUMN delta_m TO delta,
  RENAME COLUMN qty_after_m TO qty_after,
  ADD CONSTRAINT chk_movements_delta CHECK (delta <> 0);

-- ── stock_transfer_items ──────────────────────────────────────────────────────
ALTER TABLE stock_transfer_items
  DROP CHECK chk_transfer_items_qty,
  RENAME COLUMN qty_m TO qty,
  ADD CONSTRAINT chk_transfer_items_qty CHECK (qty > 0);

-- ── supplier_invoice_items ────────────────────────────────────────────────────
ALTER TABLE supplier_invoice_items
  DROP CHECK chk_sinv_items_qty,
  RENAME COLUMN qty_m TO qty,
  ADD CONSTRAINT chk_sinv_items_qty CHECK (qty > 0 AND unit_cost >= 0);

-- ── stock_opname_lines ────────────────────────────────────────────────────────
ALTER TABLE stock_opname_lines
  DROP CHECK chk_opname_lines_counted,
  RENAME COLUMN system_qty_m TO system_qty,
  RENAME COLUMN counted_qty_m TO counted_qty,
  ADD CONSTRAINT chk_opname_lines_counted CHECK (counted_qty IS NULL OR counted_qty >= 0);
