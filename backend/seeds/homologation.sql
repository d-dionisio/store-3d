\set ON_ERROR_STOP on

-- Catálogo fictício exclusivo para homologação. Não usar em produção.
BEGIN;

-- Defesa em profundidade: não confiar somente na URL fornecida ao psql.
DO $$
BEGIN
    IF current_database() <> 'loja_3d_test' THEN
        RAISE EXCEPTION 'homologation seed requires database loja_3d_test';
    END IF;
    IF current_user <> 'formo3d_test' THEN
        RAISE EXCEPTION 'homologation seed requires role formo3d_test';
    END IF;
END;
$$;

INSERT INTO categories (id, name, slug, description, sort_order, is_active)
VALUES (
    '91000000-0000-4000-8000-000000000001',
    'Decoração',
    'decoracao-hml',
    'Categoria fictícia exclusiva para homologação do checkout.',
    900,
    TRUE
)
ON CONFLICT DO NOTHING;

INSERT INTO materials (id, code, name, description, is_active)
VALUES (
    '92000000-0000-4000-8000-000000000001',
    'PLA-HML',
    'PLA Homologação',
    'Material PLA demonstrativo para homologação.',
    TRUE
)
ON CONFLICT DO NOTHING;

INSERT INTO colors (id, slug, name, hex_code, is_active)
VALUES
    ('93000000-0000-4000-8000-000000000001', 'areia-hml', 'Areia', '#E8E1D5', TRUE),
    ('93000000-0000-4000-8000-000000000002', 'verde-salvia-hml', 'Verde Sálvia', '#A3B18A', TRUE)
ON CONFLICT DO NOTHING;

INSERT INTO products (
    id, category_id, slug, name, short_description, description,
    status, is_featured, production_type
)
VALUES (
    '94000000-0000-4000-8000-000000000001',
    '91000000-0000-4000-8000-000000000001',
    'vaso-decorativo-geometrico-hml',
    'Vaso Decorativo Geométrico',
    'Produto fictício para homologação do fluxo de compra.',
    'Produto fictício de homologação, produzido sob encomenda em PLA. Não representa especificação comercial final.',
    'active',
    FALSE,
    'made_to_order'
)
ON CONFLICT DO NOTHING;

-- Peso e dimensões são valores demonstrativos para permitir testar cotação
-- de frete; devem ser substituídos por medidas reais antes de uso comercial.
INSERT INTO product_variants (
    id, product_id, material_id, color_id, size_id, finish_id, sku, name,
    price_cents, stock_quantity, reserved_quantity, track_inventory,
    weight_grams, width_mm, height_mm, depth_mm,
    production_lead_time_min_days, production_lead_time_max_days, status
)
VALUES
    (
        '95000000-0000-4000-8000-000000000001',
        '94000000-0000-4000-8000-000000000001',
        '92000000-0000-4000-8000-000000000001',
        '93000000-0000-4000-8000-000000000001',
        NULL, NULL, 'HML-VASO-GEO-PLA-ARE', 'Areia',
        4990, 0, 0, FALSE,
        500, 160, 180, 160,
        5, 8, 'active'
    ),
    (
        '95000000-0000-4000-8000-000000000002',
        '94000000-0000-4000-8000-000000000001',
        '92000000-0000-4000-8000-000000000001',
        '93000000-0000-4000-8000-000000000002',
        NULL, NULL, 'HML-VASO-GEO-PLA-VSA', 'Verde Sálvia',
        4990, 0, 0, FALSE,
        500, 160, 180, 160,
        5, 8, 'active'
    )
ON CONFLICT DO NOTHING;

-- DO NOTHING evita sobrescrita; estas verificações fazem qualquer colisão
-- de UUID, slug, nome, código ou SKU abortar a transação se os dados não
-- corresponderem integralmente ao seed esperado.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM categories
        WHERE id = '91000000-0000-4000-8000-000000000001'
          AND name = 'Decoração' AND slug = 'decoracao-hml'
          AND description = 'Categoria fictícia exclusiva para homologação do checkout.'
          AND sort_order = 900 AND is_active AND deleted_at IS NULL
    ) THEN RAISE EXCEPTION 'homologation category conflict'; END IF;

    IF NOT EXISTS (
        SELECT 1 FROM materials
        WHERE id = '92000000-0000-4000-8000-000000000001'
          AND code = 'PLA-HML' AND name = 'PLA Homologação'
          AND description = 'Material PLA demonstrativo para homologação.' AND is_active
    ) THEN RAISE EXCEPTION 'homologation material conflict'; END IF;

    IF NOT EXISTS (
        SELECT 1 FROM colors
        WHERE id = '93000000-0000-4000-8000-000000000001'
          AND slug = 'areia-hml' AND name = 'Areia' AND hex_code = '#E8E1D5' AND is_active
    ) OR NOT EXISTS (
        SELECT 1 FROM colors
        WHERE id = '93000000-0000-4000-8000-000000000002'
          AND slug = 'verde-salvia-hml' AND name = 'Verde Sálvia'
          AND hex_code = '#A3B18A' AND is_active
    ) THEN RAISE EXCEPTION 'homologation color conflict'; END IF;

    IF NOT EXISTS (
        SELECT 1 FROM products
        WHERE id = '94000000-0000-4000-8000-000000000001'
          AND category_id = '91000000-0000-4000-8000-000000000001'
          AND slug = 'vaso-decorativo-geometrico-hml'
          AND name = 'Vaso Decorativo Geométrico'
          AND short_description = 'Produto fictício para homologação do fluxo de compra.'
          AND description = 'Produto fictício de homologação, produzido sob encomenda em PLA. Não representa especificação comercial final.'
          AND status = 'active' AND NOT is_featured
          AND production_type = 'made_to_order' AND deleted_at IS NULL
    ) THEN RAISE EXCEPTION 'homologation product conflict'; END IF;

    IF (SELECT count(*) FROM product_variants
        WHERE id IN (
            '95000000-0000-4000-8000-000000000001',
            '95000000-0000-4000-8000-000000000002'
        )
          AND product_id = '94000000-0000-4000-8000-000000000001'
          AND material_id = '92000000-0000-4000-8000-000000000001'
          AND price_cents = 4990 AND size_id IS NULL AND finish_id IS NULL
          AND stock_quantity = 0 AND reserved_quantity = 0 AND NOT track_inventory
          AND weight_grams = 500 AND width_mm = 160 AND height_mm = 180 AND depth_mm = 160
          AND production_lead_time_min_days = 5 AND production_lead_time_max_days = 8
          AND status = 'active' AND deleted_at IS NULL
          AND (
            (id = '95000000-0000-4000-8000-000000000001'
             AND color_id = '93000000-0000-4000-8000-000000000001'
             AND sku = 'HML-VASO-GEO-PLA-ARE' AND name = 'Areia')
            OR
            (id = '95000000-0000-4000-8000-000000000002'
             AND color_id = '93000000-0000-4000-8000-000000000002'
             AND sku = 'HML-VASO-GEO-PLA-VSA' AND name = 'Verde Sálvia')
          )) <> 2
    THEN RAISE EXCEPTION 'homologation variant conflict'; END IF;
END;
$$;

COMMIT;
