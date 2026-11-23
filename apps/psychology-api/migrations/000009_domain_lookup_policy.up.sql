-- 000009_domain_lookup_policy.up.sql
-- Allow public and runtime lookup of verified domains under Row Level Security

CREATE POLICY domain_lookup ON domains
    FOR SELECT
    USING (is_verified = true);
