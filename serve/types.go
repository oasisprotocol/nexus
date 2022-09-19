package main

type BlockRow struct {
	Height          int64  `json:"height"`
	Hash            string `json:"hash"`
	Timestamp       int64  `json:"timestamp"`
	NumTransactions int    `json:"num_transactions"`
	SizeBytes       int64  `json:"size_bytes"`
	GasUsed         int64  `json:"gas_used"`
}

type TransactionRow struct {
	Height    int64   `json:"height"`
	Index     int     `json:"index"`
	Hash      string  `json:"hash"`
	EthHash   *string `json:"eth_hash"`
	Timestamp int64   `json:"timestamp"`
	From      string  `json:"from"`
	FeeAmount string  `json:"fee_amount"`
	FeeGas    int64   `json:"fee_gas"`
	Method    string  `json:"method"`
	To        string  `json:"to"`
	Amount    string  `json:"amount"`
}
