CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    full_name TEXT NOT NULL,
    phone TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked', 'anonymized')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(email)) > 3),
    CHECK (length(btrim(full_name)) > 0)
);

CREATE UNIQUE INDEX users_email_lower_key ON users(lower(email));

CREATE TABLE addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT,
    recipient_name TEXT NOT NULL,
    postal_code TEXT NOT NULL,
    street TEXT NOT NULL,
    number TEXT NOT NULL,
    complement TEXT,
    district TEXT NOT NULL,
    city TEXT NOT NULL,
    state_code TEXT NOT NULL CHECK (state_code ~ '^[A-Z]{2}$'),
    country_code TEXT NOT NULL DEFAULT 'BR' CHECK (country_code ~ '^[A-Z]{2}$'),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(recipient_name)) > 0),
    CHECK (length(btrim(postal_code)) > 0),
    CHECK (length(btrim(street)) > 0),
    CHECK (length(btrim(number)) > 0),
    CHECK (length(btrim(district)) > 0),
    CHECK (length(btrim(city)) > 0)
);

CREATE UNIQUE INDEX addresses_one_default_per_user_idx
    ON addresses(user_id) WHERE is_default;

