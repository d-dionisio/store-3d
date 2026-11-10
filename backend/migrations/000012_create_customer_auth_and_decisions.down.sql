DROP TABLE IF EXISTS customer_audit_events;
DROP TABLE IF EXISTS quote_proposal_decisions;
DROP TABLE IF EXISTS customer_sessions;
DROP TABLE IF EXISTS customer_magic_links;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
