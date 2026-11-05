-- NetSuite (token-based authentication) as a journal destination.

-- +goose Up
ALTER TABLE gl_connections DROP CONSTRAINT IF EXISTS gl_connections_provider_check;
ALTER TABLE gl_connections ADD CONSTRAINT gl_connections_provider_check CHECK (provider IN ('xero', 'quickbooks', 'netsuite'));

-- +goose Down
DELETE FROM gl_connections WHERE provider = 'netsuite';
ALTER TABLE gl_connections DROP CONSTRAINT IF EXISTS gl_connections_provider_check;
ALTER TABLE gl_connections ADD CONSTRAINT gl_connections_provider_check CHECK (provider IN ('xero', 'quickbooks'));
