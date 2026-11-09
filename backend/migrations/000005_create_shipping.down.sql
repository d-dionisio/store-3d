DROP TRIGGER IF EXISTS shipment_items_quantity_trigger ON shipment_items;
DROP FUNCTION IF EXISTS enforce_shipment_item_quantity();
DROP TABLE IF EXISTS shipment_items;
DROP TABLE IF EXISTS shipments;
DROP TABLE IF EXISTS order_shipping;
DROP TABLE IF EXISTS shipping_quotes;
DROP TABLE IF EXISTS shipping_services;

