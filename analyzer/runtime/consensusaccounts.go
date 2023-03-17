package runtime

import (
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
)

// queueConsensusAccountsEvents expands `batch` with DB queries that record the
// "effects" of events from the accounts module, such as dead reckoning.
//
// It does not insert the events themselves; that is done in a
// module-independent way. It only records effects for which an understanding of
// the module's semantics is necessary.
func (m *Main) queueConsensusAccountsEvents(batch *storage.QueryBatch, blockData *BlockData) {
	allEvents := append(blockData.EventData, blockData.NonTxEvents...) //nolint: gocritic
	for _, event := range allEvents {
		if event.WithScope.ConsensusAccounts == nil {
			continue
		}
		if e := event.WithScope.ConsensusAccounts.Deposit; e != nil {
			m.queueDeposit(batch, blockData.Header.Round, *e)
		}
		if e := event.WithScope.ConsensusAccounts.Withdraw; e != nil {
			m.queueWithdraw(batch, blockData.Header.Round, *e)
		}
	}
}

func (m *Main) queueDeposit(batch *storage.QueryBatch, round uint64, e consensusaccounts.DepositEvent) {
	if !e.Amount.Denomination.IsNative() {
		// Should never happen.
		m.logger.Error("non-native denomination in deposit; pretending it is native",
			"denomination", e.Amount.Denomination,
			"round", round,
			"from", e.From.String(),
			"to", e.To.String())
	}
	errorModule, errorCode := decomposeError(e.Error)
	batch.Queue(
		queries.RuntimeDepositInsert,
		m.runtime,
		round,
		e.From.String(),
		e.To.String(),
		e.Amount.Amount.String(),
		e.Nonce,
		errorModule,
		errorCode,
	)
	// Do not increase the recipient's runtime balance at this point;
	// the deposit will trigger a mint event in the runtime, and we'll update the balance then.
}

func (m *Main) queueWithdraw(batch *storage.QueryBatch, round uint64, e consensusaccounts.WithdrawEvent) {
	if !e.Amount.Denomination.IsNative() {
		// Should never happen.
		m.logger.Error("non-native denomination in withdraw; pretending it is native",
			"denomination", e.Amount.Denomination,
			"round", round,
			"from", e.From.String(),
			"to", e.To.String())
	}
	errorModule, errorCode := decomposeError(e.Error)
	batch.Queue(
		queries.RuntimeWithdrawInsert,
		m.runtime,
		round,
		e.From.String(),
		e.To.String(),
		e.Amount.Amount.String(),
		e.Nonce,
		errorModule,
		errorCode,
	)
	// Do not decrease the recipient's runtime balance at this point;
	// the withdraw will trigger a burn event in the runtime, and we'll update the balance then.
}

func decomposeError(err *consensusaccounts.ConsensusError) (*string, *uint32) {
	if err == nil {
		return nil, nil
	}

	return &err.Module, &err.Code
}
