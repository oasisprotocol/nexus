package modules

import (
	"context"
	"fmt"

	"github.com/oasislabs/oasis-indexer/storage"
)

// ConsensusAccountsHandler implements support for transforming and inserting data from the
// `consensus_accounts` module for a runtime into target storage.
type ConsensusAccountsHandler struct {
	source storage.RuntimeSourceStorage
}

// NewConsensusAccountsHandler creates a new handler for `consensus_accounts` module data.
func NewConsensusAccountsHandler(source storage.RuntimeSourceStorage) *ConsensusAccountsHandler {
	return &ConsensusAccountsHandler{source}
}

// PrepareConsensusAccountsData prepares raw data from the `consensus_accounts` module for insertion
// into target storage.
func (a *ConsensusAccountsHandler) PrepareConsensusAccountsData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := a.source.ConsensusAccountsData(ctx, round)
	if err != nil {
		return err
	}

	for _, deposit := range data.Deposits {
		fmt.Printf("deposit from %s to %s of %s\n", deposit.From.String(), deposit.To.String(), deposit.Amount.String())
	}
	return nil
}
