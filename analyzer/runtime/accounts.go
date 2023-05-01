package runtime

import (
	"math/big"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"

	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

// queueAccountsEvents expands `batch` with DB queries that record the "effects"
// of events from the accounts module, such as dead reckoning.
//
// It does not insert the events themselves; that is done in a
// module-independent way. It only records effects for which an understanding of
// the module's semantics is necessary.
func (m *Main) queueAccountsEvents(batch *storage.QueryBatch, blockData *BlockData) {
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

func (m *Main) queueMint(batch *storage.QueryBatch, round uint64, e accounts.MintEvent) {
	// Record the event.
	batch.Queue(
		queries.RuntimeMintInsert,
		m.cfg.RuntimeName,
		round,
		e.Owner.String(),
		m.StringifyDenomination(e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Increase minter's balance.
	batch.Queue(
		queries.RuntimeNativeBalanceUpdate,
		m.cfg.RuntimeName,
		e.Owner.String(),
		m.StringifyDenomination(e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
}

func (m *Main) queueBurn(batch *storage.QueryBatch, round uint64, e accounts.BurnEvent) {
	// Record the event.
	batch.Queue(
		queries.RuntimeBurnInsert,
		m.cfg.RuntimeName,
		round,
		e.Owner.String(),
		m.StringifyDenomination(e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Decrease burner's balance.
	batch.Queue(
		queries.RuntimeNativeBalanceUpdate,
		m.cfg.RuntimeName,
		e.Owner.String(),
		m.StringifyDenomination(e.Amount.Denomination),
		(&big.Int{}).Neg(e.Amount.Amount.ToBigInt()).String(),
	)
}

func (m *Main) queueTransfer(batch *storage.QueryBatch, round uint64, e accounts.TransferEvent) {
	// Record the event.
	batch.Queue(
		queries.RuntimeTransferInsert,
		m.cfg.RuntimeName,
		round,
		e.From.String(),
		e.To.String(),
		m.StringifyDenomination(e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Increase receiver's balance.
	batch.Queue(
		queries.RuntimeNativeBalanceUpdate,
		m.cfg.RuntimeName,
		e.To.String(),
		m.StringifyDenomination(e.Amount.Denomination),
		e.Amount.Amount.String(),
	)
	// Decrease sender's balance.
	batch.Queue(
		queries.RuntimeNativeBalanceUpdate,
		m.cfg.RuntimeName,
		e.From.String(),
		m.StringifyDenomination(e.Amount.Denomination),
		(&big.Int{}).Neg(e.Amount.Amount.ToBigInt()).String(),
	)
}
