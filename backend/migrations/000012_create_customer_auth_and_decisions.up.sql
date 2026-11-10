ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

CREATE TABLE customer_magic_links (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), email TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE CHECK(token_hash ~ '^[0-9a-f]{64}$'),
 expires_at TIMESTAMPTZ NOT NULL, consumed_at TIMESTAMPTZ, requested_ip INET, created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(expires_at>created_at)
);
CREATE INDEX customer_magic_links_email_created_idx ON customer_magic_links(lower(email),created_at DESC);
CREATE INDEX customer_magic_links_active_idx ON customer_magic_links(token_hash,expires_at) WHERE consumed_at IS NULL;

CREATE TABLE customer_sessions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token_hash TEXT NOT NULL UNIQUE CHECK(token_hash ~ '^[0-9a-f]{64}$'), csrf_token_hash TEXT NOT NULL CHECK(csrf_token_hash ~ '^[0-9a-f]{64}$'),
 ip_address INET, user_agent TEXT, expires_at TIMESTAMPTZ NOT NULL, last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(), revoked_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), CHECK(expires_at>created_at)
);
CREATE INDEX customer_sessions_active_idx ON customer_sessions(token_hash,expires_at) WHERE revoked_at IS NULL;
CREATE INDEX customer_sessions_user_idx ON customer_sessions(user_id,created_at DESC);

CREATE TABLE quote_proposal_decisions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE RESTRICT,
 proposal_id UUID NOT NULL REFERENCES quote_proposals(id) ON DELETE RESTRICT, user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 decision TEXT NOT NULL CHECK(decision IN('accepted','rejected')), decided_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(quote_id), UNIQUE(proposal_id), CHECK(length(decision)>0)
);
CREATE INDEX quote_proposal_decisions_user_idx ON quote_proposal_decisions(user_id,decided_at DESC);

CREATE TABLE customer_audit_events (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id UUID REFERENCES users(id) ON DELETE SET NULL,
 session_id UUID REFERENCES customer_sessions(id) ON DELETE SET NULL, action TEXT NOT NULL, resource_type TEXT,
 resource_id TEXT, outcome TEXT NOT NULL CHECK(outcome IN('success','denied','failure')), ip_address INET,
 metadata JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), CHECK(length(btrim(action))>0)
);
CREATE INDEX customer_audit_user_created_idx ON customer_audit_events(user_id,created_at DESC);
CREATE INDEX customer_audit_resource_idx ON customer_audit_events(resource_type,resource_id,created_at DESC);
