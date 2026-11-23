-- infrastructure/psychology/initdb/01-roles.sql
-- Database roles for psychology platform with restricted privileges

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'app_user') THEN
        CREATE ROLE app_user WITH LOGIN PASSWORD 'secret_app_pass' NOSUPERUSER NOBYPASSRLS;
    ELSE
        ALTER ROLE app_user WITH PASSWORD 'secret_app_pass' NOSUPERUSER NOBYPASSRLS;
    END IF;

    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'lookup_user') THEN
        CREATE ROLE lookup_user WITH LOGIN PASSWORD 'secret_lookup_pass' NOSUPERUSER NOBYPASSRLS;
    ELSE
        ALTER ROLE lookup_user WITH PASSWORD 'secret_lookup_pass' NOSUPERUSER NOBYPASSRLS;
    END IF;

    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'worker_user') THEN
        CREATE ROLE worker_user WITH LOGIN PASSWORD 'secret_worker_pass' NOSUPERUSER NOBYPASSRLS;
    ELSE
        ALTER ROLE worker_user WITH PASSWORD 'secret_worker_pass' NOSUPERUSER NOBYPASSRLS;
    END IF;
END $$;
