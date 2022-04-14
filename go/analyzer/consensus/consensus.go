// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	"golang.org/x/sync/errgroup"

	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	analyzerName = "consensus"
)

// ConsensusAnalyzer is an Analyzer for the consensus layer.
type ConsensusAnalyzer struct {
	source storage.SourceStorage
	target storage.TargetStorage
	logger *log.Logger
}

// NewConsensusAnalyzer returns a new analyzer for the consensus layer.
func NewConsensusAnalyzer(source storage.SourceStorage, target storage.TargetStorage, logger *log.Logger) *ConsensusAnalyzer {
	return &ConsensusAnalyzer{
		source: source,
		target: target,
		logger: logger.With("analyzer", analyzerName),
	}
}

// Start starts the ConsensusAnalyzer.
func (c *ConsensusAnalyzer) Start() {
	ctx := context.Background()

	height := int64(8049957)
	for {
		c.logger.Info("processing block", "height", height)
		if err := c.processBlock(ctx, height); err != nil {
			c.logger.Warn("got error", "err", err.Error())
			continue
		}

		c.logger.Info("finished processing block")

		height += 1
	}
}

// Name returns the name of the ConsensusAnalyzer.
func (c *ConsensusAnalyzer) Name() string {
	return analyzerName
}

// processBlock processes the block at the provided block height.
func (c *ConsensusAnalyzer) processBlock(ctx context.Context, height int64) error {
	group, groupCtx := errgroup.WithContext(ctx)

	batch := &storage.QueryBatch{}

	type prepareFunc = func(context.Context, int64, *storage.QueryBatch) error
	for _, f := range []prepareFunc{
		c.prepareBlockData,
		c.prepareBeaconData,
		c.prepareRegistryData,
		c.prepareStakingData,
		c.prepareSchedulerData,
		c.prepareGovernanceData,
	} {
		func(f prepareFunc) {
			group.Go(func() error {
				if err := f(groupCtx, height, batch); err != nil {
					return err
				}
				return nil
			})
		}(f)
	}

	if err := group.Wait(); err != nil {
		return err
	}

	return c.target.SendBatch(ctx, batch)
}

// prepareBlockData adds block data queries to the batch.
func (c *ConsensusAnalyzer) prepareBlockData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	data, err := c.source.BlockData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.BlockData) error{
		c.queueBlockInserts,
		c.queueTransactionInserts,
		c.queueEventInserts,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (c *ConsensusAnalyzer) queueBlockInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	batch.Queue(blocksInsertQuery,
		data.BlockHeader.Height,
		data.BlockHeader.Hash.Hex(),
		data.BlockHeader.Time,
		data.BlockHeader.StateRoot.Namespace.String(),
		int64(data.BlockHeader.StateRoot.Version),
		data.BlockHeader.StateRoot.Type.String(),
		data.BlockHeader.StateRoot.Hash.Hex(),
	)

	return nil
}

func (c *ConsensusAnalyzer) queueTransactionInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	for _, signedTx := range data.Transactions {
		// signedTx := data.Transactions[i]
		// result := data.Results[i]

		var tx transaction.Transaction
		if err := signedTx.Open(&tx); err != nil {
			continue
		}

		// batch.Queue(transactionsInsertQuery,
		// 	data.BlockHeader.Height,
		// 	signedTx.Hash().Hex(),
		// 	i,
		// 	tx.Nonce,
		// 	tx.Fee.Amount.ToBigInt(),
		// 	tx.Fee.Gas,
		// 	tx.Method,
		// 	tx.Body,
		// 	result.Error.Module,
		// 	result.Error.Code,
		// 	result.Error.Message,
		// )
	}

	return nil
}

func (c *ConsensusAnalyzer) queueEventInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	for i := 0; i < len(data.Results); i++ {
		for j := 0; j < len(data.Results[i].Events); j++ {
			backend, ty, body, err := extractEventData(data.Results[i].Events[j])
			if err != nil {
				return err
			}

			batch.Queue(eventsInsertQuery,
				backend,
				ty,
				string(body),
				data.BlockHeader.Height,
				data.Transactions[i].Hash().Hex(),
				i,
			)
		}
	}

	return nil
}

// prepareBeaconData adds beacon data queries to the batch.
func (c *ConsensusAnalyzer) prepareBeaconData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	// No beacon data, for now.
	return nil
}

// prepareRegistryData adds registry data queries to the batch.
func (c *ConsensusAnalyzer) prepareRegistryData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	data, err := c.source.RegistryData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.RegistryData) error{
		c.queueRuntimeRegistrations,
		c.queueEntityRegistrations,
		c.queueNodeRegistrations,
		c.queueNodeUnfreezes,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (c *ConsensusAnalyzer) queueRuntimeRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
	for _, runtimeEvent := range data.RuntimeEvents {
		if runtimeEvent.Runtime != nil {
			extraData, err := json.Marshal(runtimeEvent.Runtime)
			if err != nil {
				return err
			}
			batch.Queue(runtimeRegistrationQuery,
				runtimeEvent.Runtime.ID,
				string(extraData),
			)
		}
	}

	return nil
}

func (c *ConsensusAnalyzer) queueEntityRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
	// TODO
	return nil
}

func (c *ConsensusAnalyzer) queueNodeRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
	// TODO
	return nil
}

func (c *ConsensusAnalyzer) queueNodeUnfreezes(batch *storage.QueryBatch, data *storage.RegistryData) error {
	// TODO
	return nil
}

// prepareStakingData adds staking data queries to the batch.
func (c *ConsensusAnalyzer) prepareStakingData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	data, err := c.source.StakingData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.StakingData) error{
		c.queueTransfers,
		c.queueBurns,
		c.queueEscrows,
		c.queueAllowanceChanges,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (c *ConsensusAnalyzer) queueTransfers(batch *storage.QueryBatch, data *storage.StakingData) error {
	for _, transfer := range data.Transfers {
		from := transfer.From.String()
		to := transfer.To.String()
		amount := transfer.Amount.ToBigInt()
		batch.Queue(transferFromQuery,
			from,
			amount,
		)
		batch.Queue(transferToQuery,
			to,
			amount,
		)
	}

	return nil
}

func (c *ConsensusAnalyzer) queueBurns(batch *storage.QueryBatch, data *storage.StakingData) error {
	for _, burn := range data.Burns {
		batch.Queue(burnQuery,
			burn.Owner.String(),
			burn.Amount.ToBigInt(),
		)
	}

	return nil
}

func (c *ConsensusAnalyzer) queueEscrows(batch *storage.QueryBatch, data *storage.StakingData) error {
	// for _, escrow := range data.Escrows {
	// 	if escrow.Add != nil {
	// 		owner := escrow.Add.Owner.String()
	// 		escrower := escrow.Add.Escrow.String()
	// 		amount := escrow.Add.Amount.ToBigInt()
	// 		newShares := escrow.Add.NewShares.ToBigInt()
	// 		batch.Queue(ownerBalanceQuery,
	// 			owner,
	// 			amount,
	// 		)
	// 		batch.Queue(escrowBalanceQuery,
	// 			escrower,
	// 			amount,
	// 			newShares,
	// 		)
	// 		batch.Queue(delegationsQuery,
	// 			owner,
	// 			escrower,
	// 			newShares,
	// 		)
	// 	} else if escrow.Take != nil {
	// 		batch.Queue(takeEscrowQuery,
	// 			escrow.Take.Owner.String(),
	// 			escrow.Take.Amount.ToBigInt(),
	// 		)
	// 	} else if escrow.DebondingStart != nil {
	// 		batch.Queue(debondingStartRemoveDelegationQuery,
	// 			escrow.DebondingStart.Owner.String(),
	// 			escrow.DebondingStart.Escrow.String(),
	// 		)
	// 		batch.Queue(debondingStartAddDebondingDelegationQuery,
	// 			escrow.DebondingStart.Owner.String(),
	// 			escrow.DebondingStart.Escrow.String(),
	// 			escrow.DebondingStart.DebondingShares.ToBigInt(),
	// 			escrow.DebondingStart.DebondEndTime,
	// 		)
	// 	} else if escrow.Reclaim != nil {
	// 		batch.Queue(reclaimEscrowQuery,
	// 			escrow.Reclaim.Owner.String(),
	// 			escrow.Reclaim.Escrow.String(),
	// 			escrow.Reclaim.Amount.ToBigInt(),
	// 			escrow.Reclaim.Shares.ToBigInt(),
	// 		)
	// 	}
	// }

	return nil
}

func (c *ConsensusAnalyzer) queueAllowanceChanges(batch *storage.QueryBatch, data *storage.StakingData) error {
	for _, allowanceChange := range data.AllowanceChanges {
		batch.Queue(allowanceChangeQuery,
			allowanceChange.Owner.String(),
			allowanceChange.Beneficiary.String(),
			allowanceChange.Allowance.ToBigInt(),
		)
	}

	return nil
}

// prepareSchedulerData adds scheduler data queries to the batch.
func (c *ConsensusAnalyzer) prepareSchedulerData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	data, err := c.source.SchedulerData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.SchedulerData) error{
		c.queueValidatorUpdates,
		c.queueCommitteeUpdates,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (c *ConsensusAnalyzer) queueValidatorUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	for _, validator := range data.Validators {
		batch.Queue(updateVotingPowerQuery,
			validator.ID,
			validator.VotingPower,
		)
	}

	return nil
}

func (c *ConsensusAnalyzer) queueCommitteeUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	batch.Queue(truncateCommitteesQuery)
	for namespace, committees := range data.Committees {
		runtime := namespace.String()
		for _, committee := range committees {
			kind := committee.String()
			validFor := int64(committee.ValidFor)
			for _, member := range committee.Members {
				batch.Queue(addCommitteeMemberQuery,
					member.PublicKey,
					validFor,
					runtime,
					kind,
					member.Role.String(),
				)
			}
		}
	}

	return nil
}

// prepareGovernanceData adds governance data queries to the batch.
func (c *ConsensusAnalyzer) prepareGovernanceData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	data, err := c.source.GovernanceData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.GovernanceData) error{
		c.queueSubmissions,
		c.queueExecutions,
		c.queueFinalizations,
		c.queueVotes,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (c *ConsensusAnalyzer) queueSubmissions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	for _, submission := range data.ProposalSubmissions {
		if submission.Content.Upgrade != nil {
			batch.Queue(submissionUpgradeQuery,
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.ToBigInt(),
				submission.Content.Upgrade.Handler,
				submission.Content.Upgrade.Target.ConsensusProtocol.String(),
				submission.Content.Upgrade.Target.RuntimeHostProtocol.String(),
				submission.Content.Upgrade.Target.RuntimeCommitteeProtocol.String(),
				submission.Content.Upgrade.Epoch,
				submission.CreatedAt,
				submission.ClosesAt,
			)
		} else if submission.Content.CancelUpgrade != nil {
			batch.Queue(submissionCancelUpgradeQuery,
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.ToBigInt(),
				submission.Content.CancelUpgrade.ProposalID,
				submission.CreatedAt,
				submission.ClosesAt,
			)
		}
	}

	return nil
}

func (c *ConsensusAnalyzer) queueExecutions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	for _, execution := range data.ProposalExecutions {
		batch.Queue(executionQuery,
			execution.ID,
		)
	}

	return nil
}

func (c *ConsensusAnalyzer) queueFinalizations(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	for _, finalization := range data.ProposalFinalizations {
		batch.Queue(finalizationQuery,
			finalization.ID,
			finalization.State.String(),
		)
		batch.Queue(invalidVotesQuery,
			finalization.ID,
			finalization.InvalidVotes,
		)
	}

	return nil
}

func (c *ConsensusAnalyzer) queueVotes(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	for _, vote := range data.Votes {
		batch.Queue(voteQuery,
			vote.ID,
			vote.Submitter.String(),
			vote.Vote.String(),
		)
	}

	return nil
}

// TODO: eliminate this if possible
func extractEventData(event *results.Event) (backend string, ty string, body []byte, err error) {
	if event.Staking != nil {
		backend = "staking"
		if event.Staking.Transfer != nil {
			ty = "transfer"
			body, err = json.Marshal(event.Staking.Transfer)
			return
		} else if event.Staking.Burn != nil {
			ty = "burn"
			body, err = json.Marshal(event.Staking.Burn)
			return
		} else if event.Staking.Escrow != nil {
			if event.Staking.Escrow.Add != nil {
				ty = "add"
				body, err = json.Marshal(event.Staking.Escrow.Add)
				return
			} else if event.Staking.Escrow.Take != nil {
				ty = "take"
				body, err = json.Marshal(event.Staking.Escrow.Take)
				return
			} else if event.Staking.Escrow.DebondingStart != nil {
				ty = "debonding_start"
				body, err = json.Marshal(event.Staking.Escrow.DebondingStart)
				return
			} else if event.Staking.Escrow.Reclaim != nil {
				ty = "reclaim"
				body, err = json.Marshal(event.Staking.Escrow.Reclaim)
				return
			}
		} else if event.Staking.AllowanceChange != nil {
			ty = "allowance_change"
			body, err = json.Marshal(event.Staking.AllowanceChange)
			return
		}
	} else if event.Registry != nil {
		backend = "registry"
		if event.Registry.RuntimeEvent != nil {
			ty = "runtime"
			body, err = json.Marshal(event.Registry.RuntimeEvent)
			return
		} else if event.Registry.EntityEvent != nil {
			ty = "entity"
			body, err = json.Marshal(event.Registry.EntityEvent)
			return
		} else if event.Registry.NodeEvent != nil {
			ty = "node"
			body, err = json.Marshal(event.Registry.NodeEvent)
			return
		} else if event.Registry.NodeUnfrozenEvent != nil {
			ty = "node_unfrozen"
			body, err = json.Marshal(event.Registry.NodeUnfrozenEvent)
			return
		}
	} else if event.RootHash != nil {
		backend = "roothash"
		if event.RootHash.ExecutorCommitted != nil {
			ty = "executor_committed"
			body, err = json.Marshal(event.RootHash.ExecutorCommitted)
			return
		} else if event.RootHash.ExecutionDiscrepancyDetected != nil {
			ty = "execution_discrepancy_detected"
			body, err = json.Marshal(event.RootHash.ExecutionDiscrepancyDetected)
			return
		} else if event.RootHash.Finalized != nil {
			ty = "finalized"
			body, err = json.Marshal(event.RootHash.Finalized)
			return
		}
	} else if event.Governance != nil {
		backend = "governance"
		if event.Governance.ProposalSubmitted != nil {
			ty = "proposal_submitted"
			body, err = json.Marshal(event.Governance.ProposalSubmitted)
			return
		} else if event.Governance.ProposalExecuted != nil {
			ty = "proposal_executed"
			body, err = json.Marshal(event.Governance.ProposalExecuted)
			return
		} else if event.Governance.ProposalFinalized != nil {
			ty = "proposal_finalized"
			body, err = json.Marshal(event.Governance.ProposalFinalized)
			return
		} else if event.Governance.Vote != nil {
			ty = "vote"
			body, err = json.Marshal(event.Governance.Vote)
			return
		}
	}

	return "", "", []byte{}, errors.New("cockroach: unknown event type")
}
