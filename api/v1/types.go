package v1

// RuntimeTransactionList is the API response for RuntimeListTransactions.
type RuntimeTransactionList struct {
	Transactions []RuntimeTransaction `json:"transactions"`
}

// RuntimeTransaction is the API response for RuntimeGetTransaction.
type RuntimeTransaction struct {
	Round   int64   `json:"round"`
	Index   int64   `json:"index"`
	Hash    string  `json:"hash"`
	EthHash *string `json:"eth_hash"`
	// TODO: timestamp
	Sender0   string  `json:"sender_0"`
	Nonce0    uint64  `json:"nonce_0"`
	FeeAmount string  `json:"fee_amount"`
	FeeGas    uint64  `json:"fee_gas"`
	Method    string  `json:"method"`
	Body      []byte  `json:"body"`
	To        *string `json:"to"`
	Amount    *string `json:"amount"`
	Success   bool    `json:"success"`
}
