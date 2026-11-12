\set ON_ERROR_STOP on

-- Dados exclusivamente demonstrativos para desenvolvimento local.
BEGIN;

INSERT INTO categories (id, name, slug, description, sort_order, is_active) VALUES
('10000000-0000-0000-0000-000000000001', 'Decoração', 'decoracao', 'Objetos autorais para ambientes contemporâneos.', 1, TRUE),
('10000000-0000-0000-0000-000000000002', 'Natal', 'natal', 'Peças demonstrativas para celebrações natalinas.', 2, TRUE),
('10000000-0000-0000-0000-000000000003', 'Religiosos', 'religiosos', 'Símbolos produzidos sob demanda.', 3, TRUE),
('10000000-0000-0000-0000-000000000004', 'Acessórios', 'acessorios', 'Organização e utilidades para o cotidiano.', 4, TRUE)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, sort_order = EXCLUDED.sort_order, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO materials (id, code, name, description, is_active) VALUES
('20000000-0000-0000-0000-000000000001', 'PLA', 'PLA Premium', 'Biopolímero demonstrativo com acabamento fosco.', TRUE),
('20000000-0000-0000-0000-000000000002', 'PETG', 'PETG Resistente', 'Material demonstrativo para peças que exigem maior resistência.', TRUE)
ON CONFLICT (id) DO UPDATE SET code = EXCLUDED.code, name = EXCLUDED.name, description = EXCLUDED.description, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO colors (id, slug, name, hex_code, is_active) VALUES
('30000000-0000-0000-0000-000000000001', 'marfim', 'Marfim', '#F7F5F0', TRUE),
('30000000-0000-0000-0000-000000000002', 'areia', 'Areia', '#E8E1D5', TRUE),
('30000000-0000-0000-0000-000000000003', 'grafite', 'Grafite', '#17171D', TRUE),
('30000000-0000-0000-0000-000000000004', 'violeta', 'Violeta', '#8054E9', TRUE)
ON CONFLICT (id) DO UPDATE SET slug = EXCLUDED.slug, name = EXCLUDED.name, hex_code = EXCLUDED.hex_code, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO product_sizes (id, code, name, width_mm, height_mm, depth_mm, is_active) VALUES
('40000000-0000-0000-0000-000000000001', 'M', 'Médio', 180, 240, 180, TRUE),
('40000000-0000-0000-0000-000000000002', 'U', 'Único', NULL, NULL, NULL, TRUE)
ON CONFLICT (id) DO UPDATE SET code = EXCLUDED.code, name = EXCLUDED.name, width_mm = EXCLUDED.width_mm, height_mm = EXCLUDED.height_mm, depth_mm = EXCLUDED.depth_mm, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO finishes (id, code, name, description, is_active) VALUES
('50000000-0000-0000-0000-000000000001', 'FOSCO', 'Fosco', 'Acabamento demonstrativo de baixa reflexão.', TRUE)
ON CONFLICT (id) DO UPDATE SET code = EXCLUDED.code, name = EXCLUDED.name, description = EXCLUDED.description, is_active = EXCLUDED.is_active, updated_at = now();

INSERT INTO products (id, category_id, slug, name, short_description, description, status, is_featured, production_type) VALUES
('60000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'vaso-orbita', 'Vaso Órbita', 'Curvas contínuas para composições contemporâneas.', 'Vaso decorativo fictício produzido sob demanda em impressão 3D.', 'active', TRUE, 'made_to_order'),
('60000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', 'luminaria-aurora', 'Luminária Aurora', 'Luz suave através de camadas geométricas.', 'Luminária decorativa fictícia; componentes elétricos não estão incluídos nesta demonstração.', 'active', TRUE, 'made_to_order'),
('60000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000002', 'estrela-natalina', 'Estrela Natalina', 'Ornamento leve para árvore ou mesa.', 'Decoração natalina fictícia impressa em PLA.', 'active', FALSE, 'stocked'),
('60000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000003', 'presepio-luz', 'Presépio Luz', 'Silhueta minimalista e delicada.', 'Artigo religioso fictício produzido com respeito e acabamento fosco.', 'active', TRUE, 'made_to_order'),
('60000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000004', 'organizador-nexo', 'Organizador Nexo', 'Módulos integrados para a mesa.', 'Organizador fictício para acessórios e itens de escritório.', 'active', TRUE, 'stocked')
ON CONFLICT (id) DO UPDATE SET category_id = EXCLUDED.category_id, slug = EXCLUDED.slug, name = EXCLUDED.name, short_description = EXCLUDED.short_description, description = EXCLUDED.description, status = EXCLUDED.status, is_featured = EXCLUDED.is_featured, production_type = EXCLUDED.production_type, updated_at = now();

INSERT INTO product_variants (
    id, product_id, material_id, color_id, size_id, finish_id, sku, name,
    price_cents, stock_quantity, reserved_quantity, track_inventory,
    weight_grams, width_mm, height_mm, depth_mm,
    production_lead_time_min_days, production_lead_time_max_days, status
) VALUES
('70000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', '40000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000001', 'VASO-ORB-PLA-MAR', 'Marfim', 18900, 0, 0, FALSE, 420, 180, 240, 180, 5, 8, 'active'),
('70000000-0000-0000-0000-000000000002', '60000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000003', '40000000-0000-0000-0000-000000000001', '50000000-0000-0000-0000-000000000001', 'VASO-ORB-PLA-GRA', 'Grafite', 19900, 0, 0, FALSE, 420, 180, 240, 180, 5, 8, 'active'),
('70000000-0000-0000-0000-000000000003', '60000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000002', '40000000-0000-0000-0000-000000000002', '50000000-0000-0000-0000-000000000001', 'LUM-AUR-PLA-ARE', 'Areia', 24900, 0, 0, FALSE, 610, 160, 260, 160, 6, 10, 'active'),
('70000000-0000-0000-0000-000000000004', '60000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000004', '40000000-0000-0000-0000-000000000002', '50000000-0000-0000-0000-000000000001', 'EST-NAT-PLA-VIO', 'Violeta', 3900, 12, 2, TRUE, 45, 100, 100, 20, 2, 4, 'active'),
('70000000-0000-0000-0000-000000000005', '60000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', '40000000-0000-0000-0000-000000000002', '50000000-0000-0000-0000-000000000001', 'PRE-LUZ-PLA-MAR', 'Marfim', 14900, 0, 0, FALSE, 280, 220, 200, 60, 4, 7, 'active'),
('70000000-0000-0000-0000-000000000006', '60000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000003', '40000000-0000-0000-0000-000000000002', '50000000-0000-0000-0000-000000000001', 'ORG-NEX-PET-GRA', 'Grafite', 12900, 8, 1, TRUE, 350, 210, 120, 130, 4, 6, 'active'),
('70000000-0000-0000-0000-000000000007', '60000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000004', '40000000-0000-0000-0000-000000000002', '50000000-0000-0000-0000-000000000001', 'ORG-NEX-PET-VIO-INATIVO', 'Violeta indisponível', 100, 99, 0, TRUE, 350, 210, 120, 130, 4, 6, 'inactive')
ON CONFLICT (id) DO UPDATE SET price_cents = EXCLUDED.price_cents, stock_quantity = EXCLUDED.stock_quantity, reserved_quantity = EXCLUDED.reserved_quantity, track_inventory = EXCLUDED.track_inventory, status = EXCLUDED.status, updated_at = now();

INSERT INTO product_images (id, product_id, storage_provider, object_key, public_url, alt_text, media_type, sort_order, is_primary) VALUES
('80000000-0000-0000-0000-000000000001', '60000000-0000-0000-0000-000000000001', 'development', 'products/vaso-orbita.png', '/images/product-collection.png', 'Vaso Órbita demonstrativo em impressão 3D', 'image/png', 0, TRUE),
('80000000-0000-0000-0000-000000000002', '60000000-0000-0000-0000-000000000002', 'development', 'products/luminaria-aurora.png', '/images/luminaria-aurora-v2.png', 'Luminária Aurora acesa em acabamento areia', 'image/png', 0, TRUE),
('80000000-0000-0000-0000-000000000003', '60000000-0000-0000-0000-000000000003', 'development', 'products/estrela-natalina.png', '/images/product-collection.png', 'Estrela natalina demonstrativa', 'image/png', 0, TRUE),
('80000000-0000-0000-0000-000000000004', '60000000-0000-0000-0000-000000000004', 'development', 'products/presepio-luz.png', '/images/product-collection.png', 'Presépio Luz demonstrativo', 'image/png', 0, TRUE),
('80000000-0000-0000-0000-000000000005', '60000000-0000-0000-0000-000000000005', 'development', 'products/organizador-nexo.png', '/images/product-collection.png', 'Organizador Nexo demonstrativo', 'image/png', 0, TRUE)
ON CONFLICT (id) DO UPDATE SET public_url = EXCLUDED.public_url, alt_text = EXCLUDED.alt_text, sort_order = EXCLUDED.sort_order, is_primary = EXCLUDED.is_primary;

COMMIT;

