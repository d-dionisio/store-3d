CREATE TABLE quote_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_token_hash TEXT NOT NULL CHECK (owner_token_hash ~ '^[0-9a-f]{64}$'),
    storage_provider TEXT NOT NULL DEFAULT 'r2',
    bucket_name TEXT NOT NULL,
    object_key TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    file_extension TEXT NOT NULL CHECK (file_extension IN ('stl', '3mf')),
    media_type TEXT NOT NULL CHECK (media_type IN ('model/stl', 'application/vnd.ms-package.3dmanufacturing-3dmodel+xml')),
    expected_size_bytes BIGINT NOT NULL CHECK (expected_size_bytes > 0 AND expected_size_bytes <= 209715200),
    checksum_sha256 TEXT NOT NULL CHECK (checksum_sha256 ~ '^[0-9a-f]{64}$'),
    status TEXT NOT NULL DEFAULT 'authorized' CHECK (status IN ('authorized', 'confirmed', 'consumed', 'expired', 'failed')),
    expires_at TIMESTAMPTZ NOT NULL,
    confirmed_at TIMESTAMPTZ,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (storage_provider, bucket_name, object_key),
    CHECK (expires_at > created_at),
    CHECK (
        (status = 'authorized' AND confirmed_at IS NULL)
        OR status IN ('expired', 'failed')
        OR (status IN ('confirmed', 'consumed') AND confirmed_at IS NOT NULL)
    ),
    CHECK ((status = 'consumed') = (consumed_at IS NOT NULL))
);
CREATE INDEX quote_uploads_expiry_idx ON quote_uploads(status, expires_at) WHERE status IN ('authorized', 'confirmed');
ALTER TABLE quotes ADD COLUMN public_access_token_hash TEXT UNIQUE CHECK (public_access_token_hash ~ '^[0-9a-f]{64}$');
