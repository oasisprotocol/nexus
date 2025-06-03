BEGIN;

CREATE INDEX IF NOT EXISTS ix_validators_epoch ON history.validators (epoch);

COMMIT;
