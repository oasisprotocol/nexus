package runtime

import (
	"math/big"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"golang.org/x/exp/slices"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/queries"
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
		m.StringifyDenomination(e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Increase minter's balance.
	if !(m.mode == analyzer.FastSyncMode && slices.Contains(veryHighTrafficAccounts, e.Owner)) {
		batch.Queue(
			queries.RuntimeNativeBalanceUpsert,
			m.runtime,
			e.Owner.String(),
			m.StringifyDenomination(e.Amount.Denomination),
			e.Amount.Amount.String(),
		)
		batch.Queue(
			queries.RuntimeAccountTotalReceivedUpsert,
			m.runtime,
			e.Owner.String(),
			e.Amount.Amount.String(),
		)
	}
}

func (m *processor) queueBurn(batch *storage.QueryBatch, round uint64, e accounts.BurnEvent) {
	// Record the event.
	batch.Queue(
		queries.RuntimeBurnInsert,
		m.runtime,
		round,
		e.Owner.String(),
		m.StringifyDenomination(e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Decrease burner's balance.
	if !(m.mode == analyzer.FastSyncMode && slices.Contains(veryHighTrafficAccounts, e.Owner)) {
		batch.Queue(
			queries.RuntimeNativeBalanceUpsert,
			m.runtime,
			e.Owner.String(),
			m.StringifyDenomination(e.Amount.Denomination),
			(&big.Int{}).Neg(e.Amount.Amount.ToBigInt()).String(),
		)
		batch.Queue(
			queries.RuntimeAccountTotalSentUpsert,
			m.runtime,
			e.Owner.String(),
			e.Amount.Amount.String(),
		)
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
		m.StringifyDenomination(e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Increase receiver's balance.
	if !(m.mode == analyzer.FastSyncMode && slices.Contains(veryHighTrafficAccounts, e.To)) {
		batch.Queue(
			queries.RuntimeNativeBalanceUpsert,
			m.runtime,
			e.To.String(),
			m.StringifyDenomination(e.Amount.Denomination),
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
			m.StringifyDenomination(e.Amount.Denomination),
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
