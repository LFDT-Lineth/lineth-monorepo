-- Manual rollback of V005__add_forced_transactions_schema.sql (coordinator DB schema 5 -> 4).
--
-- NOT a Flyway migration: it lives outside db/coordinator so it is never applied automatically.
-- Only needed if the database itself must be back at V4. Running the coordinator with
-- schemaVersion = 4 against a V5 database works without it.
--
-- DESTRUCTIVE: permanently deletes all forced_transactions rows and all batches.proof_index_hash values.
-- Take a backup first, and stop the coordinator before running.
--
-- Usage:
--   psql -v ON_ERROR_STOP=1 -h <host> -U <user> -d linea_coordinator \
--     -f coordinator/persistence/scripts/undo_V005__add_forced_transactions_schema.sql

BEGIN;

-- Refuse to run unless V005 is the latest applied migration, so later migrations are not left orphaned
DO $$
DECLARE
  latest_version varchar;
BEGIN
  SELECT version INTO latest_version
  FROM schema_version
  WHERE success AND version IS NOT NULL
  ORDER BY installed_rank DESC
  LIMIT 1;

  IF latest_version IS DISTINCT FROM '005' THEN
    RAISE EXCEPTION 'Expected latest applied migration to be 005, found %. Aborting.', latest_version;
  END IF;
END $$;

-- Indexes and comments on forced_transactions are dropped with the table
DROP TABLE IF EXISTS forced_transactions;

ALTER TABLE batches DROP COLUMN IF EXISTS proof_index_hash;

-- Remove the Flyway history entry so a later migrate with target >= 5 re-applies V005
DELETE FROM schema_version WHERE version = '005';

COMMIT;