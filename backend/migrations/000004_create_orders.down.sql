DROP TRIGGER IF EXISTS order_items_source_trigger ON order_items;
DROP FUNCTION IF EXISTS ensure_order_item_source();
DROP TRIGGER IF EXISTS orders_quote_consistency_trigger ON orders;
DROP FUNCTION IF EXISTS ensure_quote_order_consistency();
DROP TABLE IF EXISTS order_status_events;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS order_addresses;
DROP TABLE IF EXISTS orders;

