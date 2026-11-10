ALTER TABLE orders DROP CONSTRAINT orders_checkout_key_hash_check;
DROP INDEX IF EXISTS orders_checkout_key_hash_key;
ALTER TABLE orders DROP COLUMN payment_readiness, DROP COLUMN checkout_key_hash;
