DROP TRIGGER IF EXISTS production_jobs_payment_trigger ON production_jobs;
DROP FUNCTION IF EXISTS ensure_production_payment_confirmed();
DROP TABLE IF EXISTS production_jobs;

