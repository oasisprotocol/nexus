package modules

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
)

const (
	consensusAccountsHandlerName = "consensus_accounts"
)

// ConsensusAccountsHandler implements support for transforming and inserting data from the
// `consensus_accounts` module for a runtime into target storage.
type ConsensusAccountsHandler struct {
	source storage.RuntimeSourceStorage
	qf     *analyzer.QueryFactory
	logger *log.Logger
}

// NewConsensusAccountsHandler creates a new handler for `consensus_accounts` module data.
func NewConsensusAccountsHandler(source storage.RuntimeSourceStorage, qf *analyzer.QueryFactory, logger *log.Logger) *ConsensusAccountsHandler {
	return &ConsensusAccountsHandler{source, qf, logger}
}

// PrepareConsensusAccountsData prepares raw data from the `consensus_accounts` module for insertion.
// into target storage.
func (h *ConsensusAccountsHandler) PrepareData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := h.source.ConsensusAccountsData(ctx, round)
	if err != nil {
		return fmt.Errorf("error retrieving consensus_accounts data: %w", err)
	}

	for _, f := range []func(*storage.QueryBatch, *storage.ConsensusAccountsData) error{
		h.queueDeposits,
		h.queueWithdraws,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

// Name returns the name of the handler.
func (h *ConsensusAccountsHandler) Name() string {
	return consensusAccountsHandlerName
}

func (h *ConsensusAccountsHandler) queueDeposits(batch *storage.QueryBatch, data *storage.ConsensusAccountsData) error {
	for _, deposit := range data.Deposits {
		if !deposit.Amount.Denomination.IsNative() {
			// Should never happen.
			h.logger.Error("non-native denomination in deposit; pretending it is native",
				"denomination", deposit.Amount.Denomination,
				"round", data.Round,
				"from", deposit.From.String(),
				"to", deposit.To.String())
		}
		errorModule, errorCode := decomposeError(deposit.Error)
		batch.Queue(
			h.qf.RuntimeDepositInsertQuery(),
			data.Round,
			deposit.From.String(),
			deposit.To.String(),
			deposit.Amount.Amount.String(),
			deposit.Nonce,
			errorModule,
			errorCode,
		)
	}

	return nil
}

func (h *ConsensusAccountsHandler) queueWithdraws(batch *storage.QueryBatch, data *storage.ConsensusAccountsData) error {
	for _, withdraw := range data.Withdraws {
		if !withdraw.Amount.Denomination.IsNative() {
			// Should never happen.
			h.logger.Error("non-native denomination in withdraw; pretending it is native",
				"denomination", withdraw.Amount.Denomination,
				"round", data.Round,
				"from", withdraw.From.String(),
				"to", withdraw.To.String())
		}
		errorModule, errorCode := decomposeError(withdraw.Error)
		batch.Queue(
			h.qf.RuntimeWithdrawInsertQuery(),
			data.Round,
			withdraw.From.String(),
			withdraw.To.String(),
			withdraw.Amount.Amount.String(),
			withdraw.Nonce,
			errorModule,
			errorCode,
		)

	}

	return nil
}

func decomposeError(err *consensusaccounts.ConsensusError) (*string, *uint32) {
	if err == nil {
		return nil, nil
	}

	return &err.Module, &err.Code
}
