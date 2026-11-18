DROP TRIGGER IF EXISTS production_run_cost_snapshot_validate ON production_run_cost_snapshots;
DROP FUNCTION IF EXISTS validate_production_cost_snapshot();
DROP TRIGGER IF EXISTS production_run_cost_snapshots_append_only ON production_run_cost_snapshots;
DROP TRIGGER IF EXISTS production_cost_profiles_append_only ON production_cost_profiles;
DROP FUNCTION IF EXISTS prevent_production_cost_history_mutation();
DROP TABLE IF EXISTS production_run_cost_snapshots;
DROP TABLE IF EXISTS production_cost_profile_current;
DROP TABLE IF EXISTS production_cost_profiles;
