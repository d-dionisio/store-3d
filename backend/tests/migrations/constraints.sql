\set ON_ERROR_STOP on

BEGIN;

DO $$
DECLARE
    category_id UUID;
    material_id UUID;
    color_id UUID;
    product_id UUID;
    other_product_id UUID;
    variant_id UUID;
    quote_id UUID;
    quote_order_id UUID;
    catalog_order_id UUID;
    custom_item_id UUID;
    catalog_item_id UUID;
    shipping_service_id UUID;
    shipping_quote_id UUID;
    order_shipping_id UUID;
    shipment_one_id UUID;
    shipment_two_id UUID;
    payment_id UUID;
    payment_attempt_id UUID;
BEGIN
    INSERT INTO categories(name, slug) VALUES ('Decoração', 'decoracao') RETURNING id INTO category_id;
    INSERT INTO materials(code, name) VALUES ('PLA', 'PLA Premium') RETURNING id INTO material_id;
    INSERT INTO colors(slug, name, hex_code) VALUES ('marfim', 'Marfim', '#F7F5F0') RETURNING id INTO color_id;

    INSERT INTO products(category_id, slug, name, description, status)
    VALUES (category_id, 'vaso-teste', 'Vaso Teste', 'Produto de teste das migrations.', 'active')
    RETURNING id INTO product_id;

    INSERT INTO products(category_id, slug, name, description, status)
    VALUES (category_id, 'produto-outro', 'Produto Outro', 'Produto usado para testar integridade.', 'active')
    RETURNING id INTO other_product_id;

    INSERT INTO product_variants(
        product_id, material_id, color_id, sku, price_cents,
        stock_quantity, production_lead_time_min_days, production_lead_time_max_days
    ) VALUES (product_id, material_id, color_id, 'VASO-PLA-MARFIM', 1000, 10, 2, 4)
    RETURNING id INTO variant_id;

    BEGIN
        INSERT INTO product_variants(
            product_id, material_id, color_id, sku, price_cents,
            stock_quantity, production_lead_time_min_days, production_lead_time_max_days
        ) VALUES (product_id, material_id, color_id, 'OUTRO-SKU', 1000, 10, 2, 4);
        RAISE EXCEPTION 'expected duplicate variant configuration to fail';
    EXCEPTION WHEN unique_violation THEN NULL;
    END;

    BEGIN
        INSERT INTO product_images(
            product_id, variant_id, storage_provider, object_key, alt_text, media_type
        ) VALUES (other_product_id, variant_id, 's3', 'invalid-image.webp', 'Imagem inválida', 'image/webp');
        RAISE EXCEPTION 'expected image/variant product mismatch to fail';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;

    INSERT INTO product_images(
        product_id, variant_id, storage_provider, object_key, alt_text, media_type, is_primary
    ) VALUES (product_id, variant_id, 's3', 'valid-image.webp', 'Imagem válida', 'image/webp', TRUE);

    INSERT INTO quotes(
        quote_number, customer_name, customer_email, status, material_id, color_id,
        requested_quantity, quoted_amount_cents, estimated_lead_time_days,
        quoted_at, valid_until, approved_at
    ) VALUES (
        'Q-TEST-001', 'Cliente Teste', 'cliente@example.com', 'approved', material_id, color_id,
        1, 2500, 7, now(), now() + interval '7 days', now()
    ) RETURNING id INTO quote_id;

    BEGIN
        INSERT INTO quote_files(
            quote_id, storage_provider, bucket_name, object_key, original_filename,
            file_extension, media_type, size_bytes, checksum_sha256, retention_until
        ) VALUES (
            quote_id, 's3', 'private', 'oversized.stl', 'oversized.stl',
            'stl', 'model/stl', 209715201, repeat('a', 64), now() + interval '30 days'
        );
        RAISE EXCEPTION 'expected oversized quote file to fail';
    EXCEPTION WHEN check_violation THEN NULL;
    END;

    INSERT INTO quote_files(
        quote_id, storage_provider, bucket_name, object_key, original_filename,
        file_extension, media_type, size_bytes, checksum_sha256, retention_until
    ) VALUES (
        quote_id, 's3', 'private', 'model.stl', 'model.stl',
        'stl', 'model/stl', 1024, repeat('a', 64), now() + interval '30 days'
    );

    INSERT INTO orders(
        order_number, quote_id, status, customer_name, customer_email,
        subtotal_cents, shipping_cents, discount_cents, total_cents
    ) VALUES (
        'O-QUOTE-001', quote_id, 'pending', 'Cliente Teste', 'cliente@example.com',
        2500, 0, 0, 2500
    ) RETURNING id INTO quote_order_id;

    BEGIN
        INSERT INTO orders(
            order_number, quote_id, status, customer_name, customer_email,
            subtotal_cents, shipping_cents, discount_cents, total_cents
        ) VALUES (
            'O-QUOTE-DUP', quote_id, 'pending', 'Cliente Teste', 'cliente@example.com',
            2500, 0, 0, 2500
        );
        RAISE EXCEPTION 'expected duplicate quote order to fail';
    EXCEPTION WHEN unique_violation THEN NULL;
    END;

    INSERT INTO order_items(
        order_id, item_type, product_name, description_snapshot,
        material_name, color_name, quantity, unit_price_cents
    ) VALUES (
        quote_order_id, 'custom', 'Peça personalizada', 'Snapshot da peça personalizada.',
        'PLA Premium', 'Marfim', 1, 2500
    ) RETURNING id INTO custom_item_id;

    INSERT INTO orders(
        order_number, status, customer_name, customer_email,
        subtotal_cents, shipping_cents, discount_cents, total_cents
    ) VALUES (
        'O-CATALOG-001', 'pending', 'Cliente Catálogo', 'catalogo@example.com',
        2000, 500, 0, 2500
    ) RETURNING id INTO catalog_order_id;

    BEGIN
        INSERT INTO order_items(
            order_id, item_type, product_name, description_snapshot,
            material_name, color_name, quantity, unit_price_cents
        ) VALUES (
            catalog_order_id, 'custom', 'Custom inválido', 'Não veio de orçamento.',
            'PLA Premium', 'Marfim', 1, 1000
        );
        RAISE EXCEPTION 'expected custom item without quote to fail';
    EXCEPTION WHEN raise_exception THEN NULL;
    END;

    INSERT INTO order_items(
        order_id, item_type, product_id, product_variant_id, product_name,
        description_snapshot, sku_snapshot, material_name, color_name,
        quantity, unit_price_cents
    ) VALUES (
        catalog_order_id, 'catalog', product_id, variant_id, 'Vaso Teste',
        'Snapshot imutável.', 'VASO-PLA-MARFIM', 'PLA Premium', 'Marfim', 2, 1000
    ) RETURNING id INTO catalog_item_id;

    BEGIN
        INSERT INTO production_jobs(order_item_id, material_id, color_id, quantity)
        VALUES (catalog_item_id, material_id, color_id, 2);
        RAISE EXCEPTION 'expected production without confirmed payment to fail';
    EXCEPTION WHEN raise_exception THEN NULL;
    END;

    INSERT INTO shipping_services(provider, external_service_code, carrier_name, service_name)
    VALUES ('melhor_envio', 'ME-TEST', 'Transportadora Teste', 'Serviço Teste')
    RETURNING id INTO shipping_service_id;

    INSERT INTO shipping_quotes(
        shipping_service_id, provider, external_quote_id, external_service_code,
        carrier_name, service_name, origin_postal_code, destination_postal_code,
        price_cents, estimated_min_days, estimated_max_days, package_data, expires_at
    ) VALUES (
        shipping_service_id, 'melhor_envio', 'QUOTE-EXT-1', 'ME-TEST',
        'Transportadora Teste', 'Serviço Teste', '01001000', '20040002',
        500, 3, 5, '{"weight_grams": 500}'::jsonb, now() + interval '1 hour'
    ) RETURNING id INTO shipping_quote_id;

    INSERT INTO order_shipping(
        order_id, shipping_quote_id, shipping_service_id, provider,
        external_service_code, external_quote_id, carrier_name, service_name,
        price_cents, estimated_min_days, estimated_max_days
    ) VALUES (
        catalog_order_id, shipping_quote_id, shipping_service_id, 'melhor_envio',
        'ME-TEST', 'QUOTE-EXT-1', 'Transportadora Teste', 'Serviço Teste', 500, 3, 5
    ) RETURNING id INTO order_shipping_id;

    INSERT INTO shipments(
        order_id, order_shipping_id, provider, carrier_name, service_name
    ) VALUES (
        catalog_order_id, order_shipping_id, 'melhor_envio', 'Transportadora Teste', 'Serviço Teste'
    ) RETURNING id INTO shipment_one_id;

    INSERT INTO shipments(
        order_id, order_shipping_id, provider, carrier_name, service_name
    ) VALUES (
        catalog_order_id, order_shipping_id, 'melhor_envio', 'Transportadora Teste', 'Serviço Teste'
    ) RETURNING id INTO shipment_two_id;

    INSERT INTO shipment_items(shipment_id, order_id, order_item_id, quantity)
    VALUES (shipment_one_id, catalog_order_id, catalog_item_id, 1);

    INSERT INTO shipment_items(shipment_id, order_id, order_item_id, quantity)
    VALUES (shipment_two_id, catalog_order_id, catalog_item_id, 1);

    BEGIN
        UPDATE shipment_items SET quantity = 2
        WHERE shipment_id = shipment_two_id AND order_item_id = catalog_item_id;
        RAISE EXCEPTION 'expected cumulative shipped quantity overflow to fail';
    EXCEPTION WHEN raise_exception THEN NULL;
    END;

    INSERT INTO payments(
        order_id, method, status, amount_cents, paid_amount_cents, paid_at
    ) VALUES (
        catalog_order_id, 'pix', 'paid', 2500, 2500, now()
    ) RETURNING id INTO payment_id;

    INSERT INTO payment_attempts(
        payment_id, provider, method, idempotency_key, external_payment_id,
        status, amount_cents, pix_expires_at
    ) VALUES (
        payment_id, 'provider_test', 'pix', 'idem-payment-1', 'payment-external-1',
        'paid', 2500, now() + interval '1 hour'
    ) RETURNING id INTO payment_attempt_id;

    INSERT INTO payment_webhook_events(
        payment_attempt_id, provider, external_event_id, event_type,
        external_payment_id, payload, payload_sha256
    ) VALUES (
        payment_attempt_id, 'provider_test', 'event-1', 'payment.paid',
        'payment-external-1', '{}'::jsonb, repeat('b', 64)
    );

    BEGIN
        INSERT INTO payment_webhook_events(
            payment_attempt_id, provider, external_event_id, event_type,
            payload, payload_sha256
        ) VALUES (
            gen_random_uuid(), 'provider_test', 'event-invalid-attempt', 'payment.paid',
            '{}'::jsonb, repeat('c', 64)
        );
        RAISE EXCEPTION 'expected invalid webhook attempt relationship to fail';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;

    INSERT INTO production_jobs(order_item_id, material_id, color_id, quantity)
    VALUES (catalog_item_id, material_id, color_id, 2);

    INSERT INTO production_jobs(order_item_id, material_id, color_id, quantity)
    VALUES (catalog_item_id, material_id, color_id, 2);

    RAISE NOTICE 'all migration constraint tests passed';
END;
$$;

ROLLBACK;

