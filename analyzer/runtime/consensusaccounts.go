package runtime

import (
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"

	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/storage"
)

// queueConsensusAccountsEvents expands `batch` with DB queries that record the
// "effects" of events from the accounts module, such as dead reckoning.
//
// It does not insert the events themselves; that is done in a
// module-independent way. It only records effects for which an understanding of
// the module's semantics is necessary.
func (m *processor) queueConsensusAccountsEvents(batch *storage.QueryBatch, blockData *BlockData) {
	for _, event := range blockData.EventData {
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

func (m *processor) queueDeposit(batch *storage.QueryBatch, round uint64, e consensusaccounts.DepositEvent) {
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

func (m *processor) queueWithdraw(batch *storage.QueryBatch, round uint64, e consensusaccounts.WithdrawEvent) {
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
