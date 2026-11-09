CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (length(btrim(name)) > 0),
    CHECK (length(btrim(slug)) > 0)
);

CREATE TABLE materials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(code)) > 0),
    CHECK (length(btrim(name)) > 0)
);

CREATE TABLE colors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL UNIQUE,
    hex_code TEXT NOT NULL CHECK (hex_code ~ '^#[0-9A-Fa-f]{6}$'),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(slug)) > 0),
    CHECK (length(btrim(name)) > 0)
);

CREATE TABLE product_sizes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL UNIQUE,
    width_mm INTEGER CHECK (width_mm > 0),
    height_mm INTEGER CHECK (height_mm > 0),
    depth_mm INTEGER CHECK (depth_mm > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(code)) > 0),
    CHECK (length(btrim(name)) > 0)
);

CREATE TABLE finishes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(code)) > 0),
    CHECK (length(btrim(name)) > 0)
);

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    short_description TEXT,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    production_type TEXT NOT NULL DEFAULT 'made_to_order' CHECK (production_type IN ('stocked', 'made_to_order')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (length(btrim(slug)) > 0),
    CHECK (length(btrim(name)) > 0),
    CHECK (length(btrim(description)) > 0)
);

CREATE TABLE product_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    material_id UUID NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    color_id UUID NOT NULL REFERENCES colors(id) ON DELETE RESTRICT,
    size_id UUID REFERENCES product_sizes(id) ON DELETE RESTRICT,
    finish_id UUID REFERENCES finishes(id) ON DELETE RESTRICT,
    sku TEXT NOT NULL UNIQUE,
    name TEXT,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    stock_quantity INTEGER NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
    reserved_quantity INTEGER NOT NULL DEFAULT 0 CHECK (reserved_quantity >= 0 AND reserved_quantity <= stock_quantity),
    track_inventory BOOLEAN NOT NULL DEFAULT TRUE,
    weight_grams INTEGER CHECK (weight_grams > 0),
    width_mm INTEGER CHECK (width_mm > 0),
    height_mm INTEGER CHECK (height_mm > 0),
    depth_mm INTEGER CHECK (depth_mm > 0),
    production_lead_time_min_days INTEGER NOT NULL DEFAULT 0 CHECK (production_lead_time_min_days >= 0),
    production_lead_time_max_days INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT product_variants_id_product_key UNIQUE (id, product_id),
    CONSTRAINT product_variants_configuration_key UNIQUE NULLS NOT DISTINCT
        (product_id, material_id, color_id, size_id, finish_id),
    CHECK (length(btrim(sku)) > 0),
    CHECK (production_lead_time_max_days >= production_lead_time_min_days)
);

CREATE TABLE product_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id UUID,
    storage_provider TEXT NOT NULL,
    object_key TEXT NOT NULL,
    public_url TEXT,
    alt_text TEXT NOT NULL,
    media_type TEXT NOT NULL CHECK (media_type IN ('image/jpeg', 'image/png', 'image/webp', 'image/avif')),
    sort_order INTEGER NOT NULL DEFAULT 0 CHECK (sort_order >= 0),
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT product_images_object_key UNIQUE (storage_provider, object_key),
    CONSTRAINT product_images_variant_product_fk
        FOREIGN KEY (variant_id, product_id)
        REFERENCES product_variants(id, product_id) ON DELETE CASCADE,
    CHECK (length(btrim(storage_provider)) > 0),
    CHECK (length(btrim(object_key)) > 0),
    CHECK (length(btrim(alt_text)) > 0)
);

CREATE UNIQUE INDEX product_images_primary_product_idx
    ON product_images(product_id)
    WHERE is_primary AND variant_id IS NULL;

CREATE UNIQUE INDEX product_images_primary_variant_idx
    ON product_images(variant_id)
    WHERE is_primary AND variant_id IS NOT NULL;

