CREATE TABLE editorial_catalogs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    eyebrow TEXT NOT NULL DEFAULT 'Edição digital',
    edition_label TEXT NOT NULL DEFAULT '',
    cover_image_id UUID REFERENCES product_images(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_at TIMESTAMPTZ,
    created_by UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    updated_by UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (slug = lower(slug) AND slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CHECK (length(slug) BETWEEN 2 AND 120),
    CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    CHECK (length(btrim(description)) BETWEEN 1 AND 2000),
    CHECK (length(btrim(eyebrow)) BETWEEN 1 AND 80),
    CHECK (length(edition_label) <= 80),
    CHECK ((status = 'draft' AND published_at IS NULL) OR (status = 'published' AND published_at IS NOT NULL))
);

CREATE UNIQUE INDEX editorial_catalogs_slug_lower_key ON editorial_catalogs(lower(slug));
CREATE INDEX editorial_catalogs_public_idx ON editorial_catalogs(published_at DESC, id) WHERE status = 'published';

CREATE TABLE editorial_catalog_products (
    catalog_id UUID NOT NULL REFERENCES editorial_catalogs(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (catalog_id, product_id),
    CONSTRAINT editorial_catalog_products_order_key UNIQUE (catalog_id, sort_order) DEFERRABLE INITIALLY IMMEDIATE
);

CREATE INDEX editorial_catalog_products_product_idx ON editorial_catalog_products(product_id);
