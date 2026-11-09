CREATE INDEX categories_active_sort_idx ON categories(sort_order, name) WHERE is_active AND deleted_at IS NULL;
CREATE INDEX products_category_status_idx ON products(category_id, status) WHERE deleted_at IS NULL;
CREATE INDEX products_featured_idx ON products(is_featured) WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX product_variants_product_status_idx ON product_variants(product_id, status) WHERE deleted_at IS NULL;
CREATE INDEX product_variants_material_idx ON product_variants(material_id);
CREATE INDEX product_variants_color_idx ON product_variants(color_id);
CREATE INDEX product_variants_size_idx ON product_variants(size_id) WHERE size_id IS NOT NULL;
CREATE INDEX product_variants_finish_idx ON product_variants(finish_id) WHERE finish_id IS NOT NULL;
CREATE INDEX product_images_product_sort_idx ON product_images(product_id, sort_order);

CREATE INDEX addresses_user_idx ON addresses(user_id);
CREATE INDEX quotes_user_created_idx ON quotes(user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX quotes_email_created_idx ON quotes(lower(customer_email), created_at DESC);
CREATE INDEX quotes_status_created_idx ON quotes(status, created_at);
CREATE INDEX quote_files_quote_idx ON quote_files(quote_id);
CREATE INDEX quote_files_security_idx ON quote_files(security_status) WHERE security_status <> 'clean';
CREATE INDEX quote_files_retention_idx ON quote_files(retention_until)
    WHERE deleted_from_storage_at IS NULL AND deletion_status <> 'legal_hold';

CREATE INDEX orders_user_created_idx ON orders(user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX orders_email_created_idx ON orders(lower(customer_email), created_at DESC);
CREATE INDEX orders_status_created_idx ON orders(status, created_at);
CREATE INDEX order_items_order_idx ON order_items(order_id);
CREATE INDEX order_status_events_order_time_idx ON order_status_events(order_id, occurred_at);

CREATE INDEX shipping_quotes_destination_expiry_idx ON shipping_quotes(destination_postal_code, expires_at);
CREATE INDEX shipping_quotes_expiry_idx ON shipping_quotes(expires_at);
CREATE INDEX shipments_order_created_idx ON shipments(order_id, created_at);
CREATE INDEX shipment_items_order_item_idx ON shipment_items(order_item_id);

CREATE INDEX payments_order_created_idx ON payments(order_id, created_at);
CREATE INDEX payments_status_created_idx ON payments(status, created_at);
CREATE INDEX payment_attempts_payment_created_idx ON payment_attempts(payment_id, created_at);
CREATE INDEX payment_status_events_payment_time_idx ON payment_status_events(payment_id, occurred_at);
CREATE INDEX payment_webhook_events_pending_idx ON payment_webhook_events(received_at)
    WHERE processing_status IN ('pending', 'failed');
CREATE INDEX payment_refunds_payment_created_idx ON payment_refunds(payment_id, created_at);

CREATE INDEX production_jobs_order_item_idx ON production_jobs(order_item_id);
CREATE INDEX production_jobs_status_schedule_idx ON production_jobs(status, scheduled_at);

