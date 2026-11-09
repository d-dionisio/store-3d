CREATE TABLE quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_number TEXT NOT NULL UNIQUE,
    user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    customer_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    customer_phone TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
        'draft', 'submitted', 'under_review', 'quoted', 'approved', 'rejected', 'expired', 'cancelled'
    )),
    material_id UUID REFERENCES materials(id) ON DELETE RESTRICT,
    color_id UUID REFERENCES colors(id) ON DELETE RESTRICT,
    requested_quantity INTEGER NOT NULL CHECK (requested_quantity > 0),
    width_mm INTEGER CHECK (width_mm > 0),
    height_mm INTEGER CHECK (height_mm > 0),
    depth_mm INTEGER CHECK (depth_mm > 0),
    notes TEXT,
    quoted_amount_cents BIGINT CHECK (quoted_amount_cents >= 0),
    currency TEXT NOT NULL DEFAULT 'BRL' CHECK (currency ~ '^[A-Z]{3}$'),
    estimated_lead_time_days INTEGER CHECK (estimated_lead_time_days > 0),
    valid_until TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    reviewed_at TIMESTAMPTZ,
    quoted_at TIMESTAMPTZ,
    approved_at TIMESTAMPTZ,
    rejected_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(quote_number)) > 0),
    CHECK (length(btrim(customer_name)) > 0),
    CHECK (length(btrim(customer_email)) > 3),
    CHECK (
        status NOT IN ('quoted', 'approved')
        OR (
            quoted_amount_cents IS NOT NULL
            AND estimated_lead_time_days IS NOT NULL
            AND quoted_at IS NOT NULL
            AND valid_until IS NOT NULL
        )
    ),
    CHECK (status <> 'approved' OR approved_at IS NOT NULL),
    CHECK (status <> 'rejected' OR rejected_at IS NOT NULL)
);

CREATE TABLE quote_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    storage_provider TEXT NOT NULL,
    bucket_name TEXT NOT NULL,
    object_key TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    file_extension TEXT NOT NULL CHECK (file_extension IN ('stl', '3mf')),
    media_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 209715200),
    checksum_sha256 TEXT NOT NULL CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
    security_status TEXT NOT NULL DEFAULT 'pending' CHECK (security_status IN (
        'pending', 'scanning', 'clean', 'rejected', 'failed'
    )),
    security_checked_at TIMESTAMPTZ,
    security_details JSONB,
    retention_until TIMESTAMPTZ NOT NULL,
    deletion_status TEXT NOT NULL DEFAULT 'retained' CHECK (deletion_status IN (
        'retained', 'scheduled', 'deleted', 'deletion_failed', 'legal_hold'
    )),
    deleted_from_storage_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT quote_files_object_key UNIQUE (storage_provider, bucket_name, object_key),
    CHECK (length(btrim(storage_provider)) > 0),
    CHECK (length(btrim(bucket_name)) > 0),
    CHECK (length(btrim(object_key)) > 0),
    CHECK (length(btrim(original_filename)) > 0),
    CHECK (retention_until > created_at),
    CHECK ((security_status IN ('clean', 'rejected', 'failed')) = (security_checked_at IS NOT NULL)),
    CHECK ((deletion_status = 'deleted') = (deleted_from_storage_at IS NOT NULL))
);

