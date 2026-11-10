ALTER TABLE order_shipping DROP CONSTRAINT IF EXISTS order_shipping_quote_order_fk;
DROP TABLE IF EXISTS order_packages;
DROP INDEX IF EXISTS shipping_quotes_order_request_idx;
DROP INDEX IF EXISTS shipping_quotes_order_expiry_idx;
ALTER TABLE shipping_quotes DROP CONSTRAINT IF EXISTS shipping_quotes_request_hash_format;
ALTER TABLE shipping_quotes DROP CONSTRAINT IF EXISTS shipping_quotes_order_id_id_key;
ALTER TABLE shipping_quotes DROP COLUMN IF EXISTS request_hash;
ALTER TABLE shipping_quotes DROP COLUMN IF EXISTS order_id;
