DROP TABLE IF EXISTS printer_maintenance_events;
DROP TABLE IF EXISTS material_spools;
DROP TABLE IF EXISTS printers;

UPDATE admin_users SET role = 'admin'
WHERE role IN ('production_operator', 'production_manager');
ALTER TABLE admin_users DROP CONSTRAINT admin_users_role_check;
ALTER TABLE admin_users ADD CONSTRAINT admin_users_role_check
    CHECK (role IN ('quote_viewer', 'admin'));

