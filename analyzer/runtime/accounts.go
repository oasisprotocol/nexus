package runtime

import (
	"math/big"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"golang.org/x/exp/slices"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage"
)

// queueAccountsEvents expands `batch` with DB queries that record the "effects"
// of events from the accounts module, such as dead reckoning.
//
// It does not insert the events themselves; that is done in a
// module-independent way. It only records effects for which an understanding of
// the module's semantics is necessary.
func (m *processor) queueAccountsEvents(batch *storage.QueryBatch, blockData *BlockData) {
	for _, event := range blockData.EventData {
		if event.WithScope.Accounts == nil {
			continue
		}
		if e := event.WithScope.Accounts.Mint; e != nil {
			m.queueMint(batch, blockData.Header.Round, *e)
		}
		if e := event.WithScope.Accounts.Burn; e != nil {
			m.queueBurn(batch, blockData.Header.Round, *e)
		}
		if e := event.WithScope.Accounts.Transfer; e != nil {
			m.queueTransfer(batch, blockData.Header.Round, *e)
		}
	}
}

// Accounts whose balance changes in most blocks.
// We don't update their balance during fast-sync, or else the db sees lots of deadlocks.
var veryHighTrafficAccounts = []sdkTypes.Address{
	accountsCommonPool.address,     // oasis1qz78phkdan64g040cvqvqpwkplfqf6tj6uwcsh30
	accountsFeeAccumulator.address, // oasis1qp3r8hgsnphajmfzfuaa8fhjag7e0yt35cjxq0u4
}

func (m *processor) queueMint(batch *storage.QueryBatch, round uint64, e accounts.MintEvent) {
	// Record the event.
	batch.Queue(
		queries.RuntimeMintInsert,
		m.runtime,
		round,
		e.Owner.String(),
		stringifyDenomination(m.sdkPT, e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Increase minter's balance.
	if !(m.mode == analyzer.FastSyncMode && slices.Contains(veryHighTrafficAccounts, e.Owner)) {
		batch.Queue(
			queries.RuntimeNativeBalanceUpsert,
			m.runtime,
			e.Owner.String(),
			stringifyDenomination(m.sdkPT, e.Amount.Denomination),
			e.Amount.Amount.String(),
		)
		if e.Amount.Denomination.IsNative() {
			batch.Queue(
				queries.RuntimeAccountTotalReceivedUpsert,
				m.runtime,
				e.Owner.String(),
				e.Amount.Amount.String(),
			)
		}
	}
}

func (m *processor) queueBurn(batch *storage.QueryBatch, round uint64, e accounts.BurnEvent) {
	// Record the event.
	batch.Queue(
		queries.RuntimeBurnInsert,
		m.runtime,
		round,
		e.Owner.String(),
		stringifyDenomination(m.sdkPT, e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Decrease burner's balance.
	if !(m.mode == analyzer.FastSyncMode && slices.Contains(veryHighTrafficAccounts, e.Owner)) {
		batch.Queue(
			queries.RuntimeNativeBalanceUpsert,
			m.runtime,
			e.Owner.String(),
			stringifyDenomination(m.sdkPT, e.Amount.Denomination),
			(&big.Int{}).Neg(e.Amount.Amount.ToBigInt()).String(),
		)
		if e.Amount.Denomination.IsNative() {
			batch.Queue(
				queries.RuntimeAccountTotalSentUpsert,
				m.runtime,
				e.Owner.String(),
				e.Amount.Amount.String(),
			)
		}
	}
}

func (m *processor) queueTransfer(batch *storage.QueryBatch, round uint64, e accounts.TransferEvent) {
	// Record the event.
	batch.Queue(
		queries.RuntimeTransferInsert,
		m.runtime,
		round,
		e.From.String(),
		e.To.String(),
		stringifyDenomination(m.sdkPT, e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Increase receiver's balance.
	if !(m.mode == analyzer.FastSyncMode && slices.Contains(veryHighTrafficAccounts, e.To)) {
		batch.Queue(
			queries.RuntimeNativeBalanceUpsert,
			m.runtime,
			e.To.String(),
			stringifyDenomination(m.sdkPT, e.Amount.Denomination),
			e.Amount.Amount.String(),
		)
		batch.Queue(
			queries.RuntimeAccountTotalReceivedUpsert,
			m.runtime,
			e.To.String(),
			e.Amount.Amount.String(),
		)
	}
	// Decrease sender's balance.
	if !(m.mode == analyzer.FastSyncMode && slices.Contains(veryHighTrafficAccounts, e.From)) {
		batch.Queue(
			queries.RuntimeNativeBalanceUpsert,
			m.runtime,
			e.From.String(),
			stringifyDenomination(m.sdkPT, e.Amount.Denomination),
			(&big.Int{}).Neg(e.Amount.Amount.ToBigInt()).String(),
		)
		batch.Queue(
			queries.RuntimeAccountTotalSentUpsert,
			m.runtime,
			e.From.String(),
			e.Amount.Amount.String(),
		)
	}
}

func (m *processor) queueConsensusAccountsEvents(batch *storage.QueryBatch, blockData *BlockData) {
	for _, event := range blockData.EventData {
		if event.WithScope.ConsensusAccounts == nil {
			continue
		}
		if e := event.WithScope.ConsensusAccounts.Deposit; e != nil {
			m.queueTransactionStatusUpdate(batch, blockData.Header.Round, "consensus.Deposit", e.From, e.Nonce, e.Error)
		}
		if e := event.WithScope.ConsensusAccounts.Withdraw; e != nil {
			m.queueTransactionStatusUpdate(batch, blockData.Header.Round, "consensus.Withdraw", e.From, e.Nonce, e.Error)
		}
		if e := event.WithScope.ConsensusAccounts.Delegate; e != nil {
			m.queueTransactionStatusUpdate(batch, blockData.Header.Round, "consensus.Delegate", e.From, e.Nonce, e.Error)
		}
		if e := event.WithScope.ConsensusAccounts.UndelegateStart; e != nil {
			// To contains the signer address.
			m.queueTransactionStatusUpdate(batch, blockData.Header.Round, "consensus.Undelegate", e.To, e.Nonce, e.Error)
		}
		// Nothing to do for 'UndelegateEnd'.
	}
}

// Updates the status of a transaction based on the event result.
// These transactions are special in the way that the actual action is executed in the next round.
func (m *processor) queueTransactionStatusUpdate(
	batch *storage.QueryBatch,
	round uint64,
	methodName string,
	sender sdkTypes.Address,
	nonce uint64,
	e *consensusaccounts.ConsensusError,
) {
	// We can only do this in slow-sync, because the event affects a transaction in previous round.
	// For fast-sync, this is handled in the FinalizeFastSync step.
	var errorModule *string
	var errorCode *uint32
	var errorMessage *string
	success := true
	if e != nil {
		errorModule = &e.Module
		errorCode = &e.Code
		// The event doesn't contain the error message, so construct a human readable one here.
		// TODO: We could try loading the error message, but Nexus currently doesn't have a mapping
		// from error codes to error messages. This can also be done on the frontend.
		errorMessage = common.Ptr("Consensus error: " + e.Module)
		success = false
	}
	switch m.mode {
	case analyzer.FastSyncMode:
		batch.Queue(
			queries.RuntimeConsensusAccountTransactionStatusUpdateFastSync,
			m.runtime,
			round,
			methodName,
			sender,
			nonce,
			success,
			errorModule,
			errorCode,
			errorMessage,
		)
	case analyzer.SlowSyncMode:
		batch.Queue(
			queries.RuntimeConsensusAccountTransactionStatusUpdate,
			m.runtime,
			round,
			methodName,
			sender,
			nonce,
			success,
			errorModule,
			errorCode,
			errorMessage,
		)
	}
}
