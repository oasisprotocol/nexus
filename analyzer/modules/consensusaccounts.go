package modules

import (
	"context"
	"fmt"

	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/storage"
)

// ConsensusAccountsHandler implements support for transforming and inserting data from the
// `consensus_accounts` module for a runtime into target storage.
type ConsensusAccountsHandler struct {
	source storage.RuntimeSourceStorage
	logger *log.Logger
}

// NewConsensusAccountsHandler creates a new handler for `consensus_accounts` module data.
func NewConsensusAccountsHandler(source storage.RuntimeSourceStorage, logger *log.Logger) *ConsensusAccountsHandler {
	return &ConsensusAccountsHandler{source, logger}
}

// PrepareConsensusAccountsData prepares raw data from the `consensus_accounts` module for insertion
// into target storage.
func (h *ConsensusAccountsHandler) PrepareData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := h.source.ConsensusAccountsData(ctx, round)
	if err != nil {
		h.logger.Error("error retrieving consensus_accounts data",
			"error", err,
		)
		return err
	}

	for _, deposit := range data.Deposits {
		h.logger.Error(fmt.Sprintf("deposit from %s to %s of %s\n", deposit.From.String(), deposit.To.String(), deposit.Amount.String()))
	}
	return nil
}
