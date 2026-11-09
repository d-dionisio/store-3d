DROP TABLE IF EXISTS quote_uploads;
ALTER TABLE quotes DROP COLUMN IF EXISTS public_access_token_hash;
