-- Add reserved addresses to accounts table
INSERT INTO oasis_3.accounts (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding)
VALUES
	('oasis1qrmufhkkyyf79s5za2r8yga9gnk4t446dcy3a5zm', 1322854212766028211, 0, 0, 0, 0, 0),
	('oasis1qqnv3peudzvekhulf8v3ht29z4cthkhy7gkxmph5', 0, 0, 0, 0, 0, 0),
	('oasis1qp65laz8zsa9a305wxeslpnkh9x4dv2h2qhjz0ec', 0, 0, 0, 0, 0, 0);
