-- Revert 002: restore textile columns, undo renames, remove commission_rate.

ALTER TABLE stock_opname_lines
  DROP CHECK chk_opname_lines_counted,
  RENAME COLUMN system_qty TO system_qty_m,
  RENAME COLUMN counted_qty TO counted_qty_m,
  ADD CONSTRAINT chk_opname_lines_counted CHECK (counted_qty_m IS NULL OR counted_qty_m >= 0);

ALTER TABLE supplier_invoice_items
  DROP CHECK chk_sinv_items_qty,
  RENAME COLUMN qty TO qty_m,
  ADD CONSTRAINT chk_sinv_items_qty CHECK (qty_m > 0 AND unit_cost >= 0);

ALTER TABLE stock_transfer_items
  DROP CHECK chk_transfer_items_qty,
  RENAME COLUMN qty TO qty_m,
  ADD CONSTRAINT chk_transfer_items_qty CHECK (qty_m > 0);

ALTER TABLE stock_movements
  DROP CHECK chk_movements_delta,
  RENAME COLUMN delta TO delta_m,
  RENAME COLUMN qty_after TO qty_after_m,
  ADD CONSTRAINT chk_movements_delta CHECK (delta_m <> 0);

ALTER TABLE order_items
  DROP CHECK chk_order_items_qty,
  RENAME COLUMN qty TO qty_m,
  ADD CONSTRAINT chk_order_items_qty CHECK (qty_m > 0);

ALTER TABLE inventory
  DROP CHECK chk_inventory_qty,
  RENAME COLUMN qty TO qty_m,
  ADD CONSTRAINT chk_inventory_qty CHECK (qty_m >= 0);

ALTER TABLE salespeople
  DROP COLUMN commission_rate;

ALTER TABLE products
  DROP CHECK chk_products_prices,
  DROP COLUMN unit,
  ADD COLUMN color    VARCHAR(50) NULL,
  ADD COLUMN material VARCHAR(80) NULL,
  ADD COLUMN width_cm SMALLINT UNSIGNED NULL,
  RENAME COLUMN min_stock TO min_stock_m,
  ADD CONSTRAINT chk_products_prices CHECK (sell_price >= 0 AND cost_price >= 0 AND min_stock_m >= 0);
