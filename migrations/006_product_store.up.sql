-- Assign each product to the store it belongs to.
-- NULL means unassigned (existing rows). New products should always have a store.
ALTER TABLE products
  ADD COLUMN store_id INT UNSIGNED NULL AFTER supplier_id,
  ADD CONSTRAINT fk_products_store FOREIGN KEY (store_id) REFERENCES stores (id);
