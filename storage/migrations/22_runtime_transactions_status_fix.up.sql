BEGIN;

-- Fix status field for transactions that had failed, but were mistakenly
-- overriden as pending.
UPDATE chain.runtime_transactions
	SET success = false
	WHERE success IS NULL
	AND method IN (
		'consensus.Deposit',
		'consensus.Delegate',
		'consensus.Withdraw',
		'consensus.Undelegate'
	)
	AND error_message_raw IS NOT NULL;

-- Fix status field for successful 'consensus.Undelegate' transactions, which
-- were not correctly handled before.
UPDATE chain.runtime_transactions tx
	SET success = true
	WHERE
		tx.method = 'consensus.Undelegate'AND
		tx.success IS NULL AND
		EXISTS (
			SELECT 1
			FROM chain.runtime_transaction_signers rts
			JOIN chain.runtime_events ev ON
				ev.runtime = tx.runtime AND
				ev.round = tx.round + 1 AND
				ev.type = 'consensus_accounts.undelegate_start' AND
				ev.body->>'to' = rts.signer_address
			WHERE
				rts.runtime = tx.runtime AND
				rts.round = tx.round AND
				rts.tx_index = tx.tx_index
		);

COMMIT;
