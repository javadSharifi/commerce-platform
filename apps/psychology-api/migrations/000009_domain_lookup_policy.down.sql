-- 000009_domain_lookup_policy.down.sql
-- Revert domain lookup policy on domains

DROP POLICY IF EXISTS domain_lookup ON domains;
