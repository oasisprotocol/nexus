// Package cockroach implements the target storage interface
// backed by CockroachDB.
package cockroach

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
)

const (
	clientName = "cockroach"
)

type CockroachClient struct {
	pool *pgxpool.Pool
}

// NewCockroachClient creates a new Coc kroachDB client.
func NewCockroachClient(connstring string) (*CockroachClient, error) {
	pool, err := pgxpool.Connect(context.Background(), connstring)
	if err != nil {
		return nil, err
	}
	return &CockroachClient{pool}, nil
}

// Connection returns a new pgx connection from the connection pool.
func (c *CockroachClient) connection(ctx context.Context) (*pgxpool.Conn, error) {
	return c.pool.Acquire(ctx)
}

// SetBlockData applies the block data as an update at the provided block height.
func (c *CockroachClient) SetBlockData(ctx context.Context, data *storage.BlockData) error {
	conn, err := c.connection(ctx)
	if err != nil {
		return err
	}

	if err := crdbpgx.ExecuteTx(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return doSetBlockData(ctx, tx, data)
	}); err != nil {
		return err
	}

	return nil
}

func doSetBlockData(ctx context.Context, tx pgx.Tx, data *storage.BlockData) error {
	batch := &pgx.Batch{}

	for _, f := range []func(*pgx.Batch, *storage.BlockData) error{
		queueBlockInserts,
		queueTransactionInserts,
		queueEventInserts,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return sendAndVerifyBatch(ctx, tx, batch)
}

func queueBlockInserts(batch *pgx.Batch, data *storage.BlockData) error {
	batch.Queue(blocksInsertQuery,
		data.BlockHeader.Height,
		data.BlockHeader.Hash,
		data.BlockHeader.Time,
		data.BlockHeader.StateRoot.Namespace.Hex(),
		int64(data.BlockHeader.StateRoot.Version),
		data.BlockHeader.StateRoot.Type.String(),
		data.BlockHeader.StateRoot.Hash,
	)

	return nil
}

func queueTransactionInserts(batch *pgx.Batch, data *storage.BlockData) error {
	for i := 0; i < len(data.Transactions); i++ {
		signedTransaction := data.Transactions[i]
		result := data.Results[i]

		var transaction transaction.Transaction
		data.Transactions[i].Open(&transaction)

		batch.Queue(transactionsInsertQuery,
			data.BlockHeader.Height,
			signedTransaction.Hash(),
			i,
			transaction.Nonce,
			transaction.Fee.Amount.ToBigInt(),
			transaction.Fee.Gas,
			transaction.Method,
			transaction.Body,
			result.Error.Module,
			result.Error.Code,
			result.Error.Message,
		)
	}

	return nil
}

func queueEventInserts(batch *pgx.Batch, data *storage.BlockData) error {
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
				data.Transactions[i].Hash(),
				i,
			)
		}
	}

	return nil
}

// SetBeaconData applies the beacon data as an update at the provided block height.
func (c *CockroachClient) SetBeaconData(ctx context.Context, data *storage.BeaconData) error {
	conn, err := c.connection(ctx)
	if err != nil {
		return err
	}

	if err := crdbpgx.ExecuteTx(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return doSetBeaconData(ctx, tx, data)
	}); err != nil {
		return err
	}

	return nil
}

func doSetBeaconData(ctx context.Context, tx pgx.Tx, _data *storage.BeaconData) error {
	batch := &pgx.Batch{}

	// No beacon data, for now.

	return sendAndVerifyBatch(ctx, tx, batch)
}

// SetRegistryData applies the registry data as an update at the provided block height.
func (c *CockroachClient) SetRegistryData(ctx context.Context, data *storage.RegistryData) error {
	conn, err := c.connection(ctx)
	if err != nil {
		return err
	}

	if err := crdbpgx.ExecuteTx(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return doSetRegistryData(ctx, tx, data)
	}); err != nil {
		return err
	}

	return nil
}

func doSetRegistryData(ctx context.Context, tx pgx.Tx, data *storage.RegistryData) error {
	batch := &pgx.Batch{}

	for _, f := range []func(*pgx.Batch, *storage.RegistryData) error{
		queueRuntimeRegistrations,
		queueEntityRegistrations,
		queueNodeRegistrations,
		queueNodeUnfreezes,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return sendAndVerifyBatch(ctx, tx, batch)
}

func queueRuntimeRegistrations(batch *pgx.Batch, data *storage.RegistryData) error {
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

func queueEntityRegistrations(batch *pgx.Batch, data *storage.RegistryData) error {
	// TODO
	return nil
}

func queueNodeRegistrations(batch *pgx.Batch, data *storage.RegistryData) error {
	// TODO
	return nil
}

func queueNodeUnfreezes(batch *pgx.Batch, data *storage.RegistryData) error {
	// TODO
	return nil
}

// SetStakingData applies the staking data as an update at the provided block height.
func (c *CockroachClient) SetStakingData(ctx context.Context, data *storage.StakingData) error {
	conn, err := c.connection(ctx)
	if err != nil {
		return err
	}

	if err := crdbpgx.ExecuteTx(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return doSetStakingData(ctx, tx, data)
	}); err != nil {
		return err
	}

	return nil
}

func doSetStakingData(ctx context.Context, tx pgx.Tx, data *storage.StakingData) error {
	batch := &pgx.Batch{}

	for _, f := range []func(*pgx.Batch, *storage.StakingData) error{
		queueTransfers,
		queueBurns,
		queueEscrows,
		queueAllowanceChanges,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return sendAndVerifyBatch(ctx, tx, batch)
}

func queueTransfers(batch *pgx.Batch, data *storage.StakingData) error {
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

func queueBurns(batch *pgx.Batch, data *storage.StakingData) error {
	for _, burn := range data.Burns {
		batch.Queue(burnQuery,
			burn.Owner.String(),
			burn.Amount.ToBigInt(),
		)
	}

	return nil
}

func queueEscrows(batch *pgx.Batch, data *storage.StakingData) error {
	for _, escrow := range data.Escrows {
		if escrow.Add != nil {
			owner := escrow.Add.Owner.String()
			escrower := escrow.Add.Escrow.String()
			amount := escrow.Add.Amount.ToBigInt()
			newShares := escrow.Add.NewShares.ToBigInt()
			batch.Queue(ownerBalanceQuery,
				owner,
				amount,
			)
			batch.Queue(escrowBalanceQuery,
				escrower,
				amount,
				newShares,
			)
			batch.Queue(delegationsQuery,
				owner,
				escrower,
				newShares,
			)
		} else if escrow.Take != nil {
			batch.Queue(takeEscrowQuery,
				escrow.Take.Owner.String(),
				escrow.Take.Amount.ToBigInt(),
			)
		} else if escrow.Reclaim != nil {
			// TODO: Reclaiming escrow is tricky. This event is emitted
			// after the debonding period ends. But you probably need to parse
			// the actual transaction to decide when the debonding period begins.
			batch.Queue(reclaimEscrowQuery,
				escrow.Reclaim.Owner.String(),
				escrow.Reclaim.Escrow.String(),
				escrow.Reclaim.Amount.ToBigInt(),
				escrow.Reclaim.Shares.ToBigInt(),
			)
		}
	}

	return nil
}

func queueAllowanceChanges(batch *pgx.Batch, data *storage.StakingData) error {
	for _, allowanceChange := range data.AllowanceChanges {
		batch.Queue(allowanceChangeQuery,
			allowanceChange.Owner.String(),
			allowanceChange.Beneficiary.String(),
			allowanceChange.Allowance.ToBigInt(),
		)
	}

	return nil
}

// SetSchedulerData applies the scheduler data as an update at the provided block height.
func (c *CockroachClient) SetSchedulerData(ctx context.Context, data *storage.SchedulerData) error {
	conn, err := c.connection(ctx)
	if err != nil {
		return err
	}

	if err := crdbpgx.ExecuteTx(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return doSetSchedulerData(ctx, tx, data)
	}); err != nil {
		return err
	}

	return nil
}

func doSetSchedulerData(ctx context.Context, tx pgx.Tx, data *storage.SchedulerData) error {
	batch := &pgx.Batch{}

	for _, f := range []func(*pgx.Batch, *storage.SchedulerData) error{
		queueValidatorUpdates,
		queueCommitteeUpdates,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return sendAndVerifyBatch(ctx, tx, batch)
}

func queueValidatorUpdates(batch *pgx.Batch, data *storage.SchedulerData) error {
	for _, validator := range data.Validators {
		batch.Queue(updateVotingPowerQuery,
			validator.ID,
			validator.VotingPower,
		)
	}

	return nil
}

func queueCommitteeUpdates(batch *pgx.Batch, data *storage.SchedulerData) error {
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

// SetGovernanceData applies the governance data as an update at the provided block height.
func (c *CockroachClient) SetGovernanceData(ctx context.Context, data *storage.GovernanceData) error {
	conn, err := c.connection(ctx)
	if err != nil {
		return err
	}

	if err := crdbpgx.ExecuteTx(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return doSetGovernanceData(ctx, tx, data)
	}); err != nil {
		return err
	}

	return nil
}

func doSetGovernanceData(ctx context.Context, tx pgx.Tx, data *storage.GovernanceData) error {
	batch := &pgx.Batch{}

	for _, f := range []func(*pgx.Batch, *storage.GovernanceData) error{
		queueSubmissions,
		queueExecutions,
		queueFinalizations,
		queueVotes,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return sendAndVerifyBatch(ctx, tx, batch)
}

func queueSubmissions(batch *pgx.Batch, data *storage.GovernanceData) error {
	// TODO: Pending storage interface update
	// for _, submission := range data.ProposalSubmissions {
	// 	if submission.Content.Upgrade != nil {
	// 		batch.Queue(submissionUpgradeQuery,
	// 			submission.ID,
	// 			submission.Submitter.String(),
	// 			submission.State.String(),
	// 			submission.Deposit.ToBigInt(),
	// 			submission.Content.Upgrade.Handler,
	// 			submission.Content.Upgrade.Target.ConsensusProtocol.String(),
	// 			submission.Content.Upgrade.Target.RuntimeHostProtocol.String(),
	// 			submission.Content.Upgrade.Target.RuntimeCommitteeProtocol.String(),
	// 			submission.Content.Upgrade.Epoch,
	// 			submission.CreatedAt,
	// 			submission.ClosesAt,
	// 		)
	// 	} else if submission.Content.CancelUpgrade != nil {
	// 		batch.Queue(submissionCancelUpgradeQuery,
	// 			submission.ID,
	// 			submission.Submitter.String(),
	// 			submission.State.String(),
	// 			submission.Deposit.ToBigInt(),
	// 			submission.Content.CancelUpgrade.ProposalID,
	// 			submission.CreatedAt,
	// 			submission.ClosesAt,
	// 		)
	// 	}
	// }

	return nil
}

func queueExecutions(batch *pgx.Batch, data *storage.GovernanceData) error {
	for _, execution := range data.ProposalExecutions {
		batch.Queue(executionQuery,
			execution.ID,
		)
	}

	return nil
}

func queueFinalizations(batch *pgx.Batch, data *storage.GovernanceData) error {
	for _, finalization := range data.ProposalFinalizations {
		batch.Queue(finalizationQuery,
			finalization.ID,
			finalization.State.String(),
		)
		// TODO: Pending storage interface update
		// batch.Queue(invalidVotesQuery,
		// 	finalization.ID,
		// 	finalization.InvalidVotes,
		// )
	}

	return nil
}

func queueVotes(batch *pgx.Batch, data *storage.GovernanceData) error {
	for _, vote := range data.Votes {
		batch.Queue(voteQuery,
			vote.ID,
			vote.Submitter.String(),
			vote.Vote.String(),
		)
	}

	return nil
}

// Name returns the name of the CockroachDB client.
func (c *CockroachClient) Name() string {
	return clientName
}

func sendAndVerifyBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) error {
	batchResults := tx.SendBatch(ctx, batch)
	defer batchResults.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := batchResults.Exec(); err != nil {
			return err
		}
	}

	return nil
}

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

// TODO: Cleanup method to gracefully shut down client.
