DROP TABLE IF EXISTS security_events;
ALTER TABLE customer_sessions DROP COLUMN IF EXISTS authentication_level;
ALTER TABLE admin_sessions DROP COLUMN IF EXISTS authentication_level;
DROP TABLE IF EXISTS customer_auth_tokens;
DROP TABLE IF EXISTS auth_challenges;
DROP TABLE IF EXISTS auth_recovery_codes;
DROP TABLE IF EXISTS auth_mfa_factors;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_password_hash_check;
ALTER TABLE users
    DROP COLUMN IF EXISTS locked_until,
    DROP COLUMN IF EXISTS failed_login_attempts,
    DROP COLUMN IF EXISTS password_changed_at,
    DROP COLUMN IF EXISTS password_hash;
