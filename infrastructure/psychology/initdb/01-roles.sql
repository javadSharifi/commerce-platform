-- infrastructure/psychology/initdb/01-roles.sql
-- Database roles for psychology platform

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'app_user') THEN
        CREATE ROLE app_user LOGIN NOSUPERUSER NOBYPASSRLS;
    END IF;
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'lookup_user') THEN
        CREATE ROLE lookup_user LOGIN NOSUPERUSER NOBYPASSRLS;
    END IF;
END $$;
