BEGIN;

UPDATE chain.runtime_transactions
	SET fee_symbol = 'EURAU'
	WHERE runtime IN ('pontusx_dev', 'pontusx_test') AND fee_symbol = 'EUROe';

UPDATE chain.runtime_transactions
	SET amount_symbol = 'EURAU'
	WHERE runtime IN ('pontusx_dev', 'pontusx_test') AND amount_symbol = 'EUROe';

UPDATE chain.runtime_transfers
	SET symbol = 'EURAU'
	WHERE runtime IN ('pontusx_dev', 'pontusx_test') AND symbol = 'EUROe';

UPDATE chain.runtime_sdk_balances
	SET symbol = 'EURAU'
	WHERE runtime IN ('pontusx_dev', 'pontusx_test') AND symbol = 'EUROe';

COMMIT;
