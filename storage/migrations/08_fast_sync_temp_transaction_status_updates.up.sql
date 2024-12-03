BEGIN;

CREATE TABLE todo_updates.transaction_status_updates( -- Tracks transaction status updates for consensus-accounts transactions.
  runtime runtime NOT NULL,
  round UINT63 NOT NULL,
  method TEXT NOT NULL,
  sender oasis_addr NOT NULL,
  nonce UINT63 NOT NULL,

  success BOOLEAN NOT NULL,
  error_module TEXT,
  error_code UINT63,
  error_message TEXT
);

COMMIT;
