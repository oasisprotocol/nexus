// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/jackc/pgx/v4"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"golang.org/x/sync/errgroup"

	"github.com/oasislabs/oasis-indexer/analyzer"
	"github.com/oasislabs/oasis-indexer/analyzer/util"
	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/metrics"
	"github.com/oasislabs/oasis-indexer/storage"
)

const (
	analyzerName = "consensus_main_damask"
)

var (
	// ErrOutOfRange is returned if the current block does not fall within tge
	// analyzer's analysis range.
	ErrOutOfRange = errors.New("range not found. no data source available")

	// ErrLatestBlockNotFound is returned if the analyzer has not indexed any
	// blocks yet. This indicates to begin from the start of its range.
	ErrLatestBlockNotFound = errors.New("latest block not found")
)

// Main is the main Analyzer for the consensus layer.
type Main struct {
	rangeCfg analyzer.RangeConfig
	target   storage.TargetStorage
	logger   *log.Logger
	metrics  metrics.DatabaseMetrics
}

// NewMain returns a new main analyzer for the consensus layer.
func NewMain(target storage.TargetStorage, logger *log.Logger) *Main {
	return &Main{
		target:  target,
		logger:  logger.With("analyzer", analyzerName),
		metrics: metrics.NewDefaultDatabaseMetrics(analyzerName),
	}
}

// SetRange adds configuration for the range of blocks to process to
// this analyzer. It is intended to be called before Start.
func (m *Main) SetRange(cfg analyzer.RangeConfig) {
	m.rangeCfg = cfg
	m.rangeCfg.ChainID = strcase.ToSnake(m.rangeCfg.ChainID)
}

// Start starts the main consensus analyzer.
func (m *Main) Start() {
	ctx := context.Background()

	// Get block to be indexed.
	var height int64

	latest, err := m.latestBlock(ctx)
	if err != nil {
		if err != pgx.ErrNoRows {
			m.logger.Error("last block height not found",
				"err", err.Error(),
			)
			return
		}
		m.logger.Debug("setting height using range config")
		height = m.rangeCfg.From
	} else {
		m.logger.Debug("setting height using latest block")
		height = latest + 1
	}

	backoff := util.NewBackoff(
		100*time.Millisecond,
		6*time.Second,
		// ^cap the maximum timeout at the expected
		// consensus block time
	)
	for {
		if err := m.processBlock(ctx, height); err != nil {
			if err == ErrOutOfRange {
				m.logger.Info("no data source available at this height",
					"height", height,
				)
				return
			}

			m.logger.Error("error processing block",
				"err", err.Error(),
			)
			backoff.Wait()
			continue
		}

		backoff.Reset()
		height++
	}
}

// Name returns the name of the Main.
func (m *Main) Name() string {
	return analyzerName
}

// source returns the source storage for the provided block height.
func (m *Main) source(height int64) (storage.SourceStorage, error) {
	r := m.rangeCfg
	if height >= r.From && (height <= r.To || r.To == 0) {
		return r.Source, nil
	}

	return nil, ErrOutOfRange
}

// latestBlock returns the latest block processed by the consensus analyzer.
func (m *Main) latestBlock(ctx context.Context) (int64, error) {
	row, err := m.target.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT height FROM %s.processed_blocks
				WHERE analyzer = $1
				ORDER BY height DESC
				LIMIT 1
		`, m.rangeCfg.ChainID),
		// ^analyzers should only analyze for a single chain ID, and we anchor this
		// at the starting block.
		analyzerName,
	)
	if err != nil {
		return 0, err
	}

	var latest int64
	if err := row.Scan(&latest); err != nil {
		return 0, err
	}
	return latest, nil
}

// processBlock processes the block at the provided block height.
func (m *Main) processBlock(ctx context.Context, height int64) error {
	m.logger.Info("processing block",
		"height", height,
	)
	chainID := m.rangeCfg.ChainID

	group, groupCtx := errgroup.WithContext(ctx)

	// Prepare and perform updates.
	batch := &storage.QueryBatch{}

	type prepareFunc = func(context.Context, int64, *storage.QueryBatch) error
	for _, f := range []prepareFunc{
		m.prepareBlockData,
		m.prepareRegistryData,
		m.prepareStakingData,
		m.prepareSchedulerData,
		m.prepareGovernanceData,
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

	// Update indexing progress.
	group.Go(func() error {
		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.processed_blocks (height, analyzer, processed_time)
			VALUES
				($1, $2, CURRENT_TIMESTAMP);
		`, chainID),
			height,
			analyzerName,
		)
		return nil
	})

	if err := group.Wait(); err != nil {
		return err
	}

	opName := "process_block"
	timer := m.metrics.DatabaseTimer(m.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := m.target.SendBatch(ctx, batch); err != nil {
		m.metrics.DatabaseCounter(m.target.Name(), opName, "failure").Inc()
		return err
	}
	m.metrics.DatabaseCounter(m.target.Name(), opName, "success").Inc()
	return nil
}

// prepareBlockData adds block data queries to the batch.
func (m *Main) prepareBlockData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.BlockData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.BlockData) error{
		m.queueBlockInserts,
		m.queueTransactionInserts,
		m.queueEventInserts,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueBlockInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	chainID := m.rangeCfg.ChainID

	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.blocks (height, block_hash, time, namespace, version, type, root_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7);
	`, chainID),
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

func (m *Main) queueTransactionInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	chainID := m.rangeCfg.ChainID

	for i := range data.Transactions {
		signedTx := data.Transactions[i]
		result := data.Results[i]

		var tx transaction.Transaction
		if err := signedTx.Open(&tx); err != nil {
			continue
		}

		sender := staking.NewAddress(
			signedTx.Signature.PublicKey,
		).String()

		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.transactions (block, txn_hash, txn_index, nonce, fee_amount, max_gas, method, sender, body, module, code, message)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);
		`, chainID),
			data.BlockHeader.Height,
			signedTx.Hash().Hex(),
			i,
			tx.Nonce,
			tx.Fee.Amount.ToBigInt().Uint64(),
			tx.Fee.Gas,
			tx.Method,
			sender,
			tx.Body,
			result.Error.Module,
			result.Error.Code,
			result.Error.Message,
		)
	}

	return nil
}

func (m *Main) queueEventInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	chainID := m.rangeCfg.ChainID

	for i := 0; i < len(data.Results); i++ {
		for j := 0; j < len(data.Results[i].Events); j++ {
			backend, ty, body, err := extractEventData(data.Results[i].Events[j])
			if err != nil {
				return err
			}

			batch.Queue(fmt.Sprintf(`
				INSERT INTO %s.events (backend, type, body, txn_block, txn_hash, txn_index)
					VALUES ($1, $2, $3, $4, $5, $6);
			`, chainID),
				backend.String(),
				ty.String(),
				string(body),
				data.BlockHeader.Height,
				data.Transactions[i].Hash().Hex(),
				i,
			)
		}
	}

	return nil
}

// prepareRegistryData adds registry data queries to the batch.
func (m *Main) prepareRegistryData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.RegistryData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.RegistryData) error{
		m.queueRuntimeRegistrations,
		m.queueEntityEvents,
		m.queueNodeRegistrations,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}
	return nil
}

func (m *Main) queueRuntimeRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
	chainID := m.rangeCfg.ChainID

	for _, runtimeEvent := range data.RuntimeEvents {
		keyManager := "none"

		if runtimeEvent.Runtime.KeyManager != nil {
			keyManager = runtimeEvent.Runtime.KeyManager.String()
		}

		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.runtimes (id, suspended, kind, tee_hardware, key_manager)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (id) DO
				UPDATE SET
					suspended = excluded.suspended,
					kind = excluded.kind,
					tee_hardware = excluded.tee_hardware,
					key_manager = excluded.key_manager;
		`, chainID),
			runtimeEvent.Runtime.ID.String(),
			false,
			runtimeEvent.Runtime.Kind.String(),
			runtimeEvent.Runtime.TEEHardware.String(),
			keyManager,
		)
	}

	return nil
}

func (m *Main) queueEntityEvents(batch *storage.QueryBatch, data *storage.RegistryData) error {
	chainID := m.rangeCfg.ChainID

	for _, entityEvent := range data.EntityEvents {
		var nodes []string
		for _, node := range entityEvent.Entity.Nodes {
			nodes = append(nodes, node.String())
		}
		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.entities (id, address) VALUES ($1, $2)
				ON CONFLICT (id) DO
				UPDATE SET
					address = excluded.address;
		`, chainID),
			entityEvent.Entity.ID.String(),
			strings.Join(nodes, ","),
		)
	}

	return nil
}

func (m *Main) queueNodeRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
	chainID := m.rangeCfg.ChainID

	for _, nodeEvent := range data.NodeEvent {
		vrfPubkey := ""

		if nodeEvent.Node.VRF != nil {
			vrfPubkey = nodeEvent.Node.VRF.ID.String()
		}
		var tlsAddresses []string
		for _, address := range nodeEvent.Node.TLS.Addresses {
			tlsAddresses = append(tlsAddresses, address.String())
		}

		var p2pAddresses []string
		for _, address := range nodeEvent.Node.P2P.Addresses {
			p2pAddresses = append(p2pAddresses, address.String())
		}

		var consensusAddresses []string
		for _, address := range nodeEvent.Node.Consensus.Addresses {
			consensusAddresses = append(consensusAddresses, address.String())
		}

		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.nodes (id, entity_id, expiration, tls_pubkey, tls_next_pubkey, tls_addresses, p2p_pubkey, p2p_addresses, consensus_pubkey, consensus_address, vrf_pubkey, roles, software_version, voting_power)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
				ON CONFLICT (id) DO
				UPDATE SET
					entity_id = excluded.entity_id,
					expiration = excluded.expiration,
					tls_pubkey = excluded.tls_pubkey,
					tls_next_pubkey = excluded.tls_next_pubkey,
					tls_addresses = excluded.tls_addresses,
					p2p_pubkey = excluded.p2p_pubkey,
					p2p_addresses = excluded.p2p_addresses,
					consensus_pubkey = excluded.consensus_pubkey,
					consensus_address = excluded.consensus_address,
					vrf_pubkey = excluded.vrf_pubkey,
					roles = excluded.roles,
					software_version = excluded.software_version,
					voting_power = excluded.voting_power;
		`, chainID),
			nodeEvent.Node.ID.String(),
			nodeEvent.Node.EntityID.String(),
			nodeEvent.Node.Expiration,
			nodeEvent.Node.TLS.PubKey.String(),
			nodeEvent.Node.TLS.NextPubKey.String(),
			fmt.Sprintf(`{'%s'}`, strings.Join(tlsAddresses, `','`)),
			nodeEvent.Node.P2P.ID.String(),
			fmt.Sprintf(`{'%s'}`, strings.Join(p2pAddresses, `','`)),
			nodeEvent.Node.Consensus.ID.String(),
			strings.Join(consensusAddresses, ","),
			vrfPubkey,
			nodeEvent.Node.Roles,
			nodeEvent.Node.SoftwareVersion,
			0,
		)
	}
	return nil
}

func (m *Main) prepareStakingData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.StakingData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.StakingData) error{
		m.queueTransfers,
		m.queueBurns,
		m.queueEscrows,
		m.queueAllowanceChanges,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueTransfers(batch *storage.QueryBatch, data *storage.StakingData) error {
	chainID := m.rangeCfg.ChainID

	for _, transfer := range data.Transfers {
		from := transfer.From.String()
		to := transfer.To.String()
		amount := transfer.Amount.ToBigInt().Uint64()
		batch.Queue(fmt.Sprintf(`
			UPDATE %s.accounts
			SET general_balance = general_balance - $2
				WHERE address = $1;
		`, chainID),
			from,
			amount,
		)
		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.accounts (address, general_balance)
				VALUES ($1, $2)
			ON CONFLICT (address) DO
				UPDATE SET general_balance = %s.accounts.general_balance - $2;
		`, chainID, chainID),
			to,
			amount,
		)
	}

	return nil
}

func (m *Main) queueBurns(batch *storage.QueryBatch, data *storage.StakingData) error {
	chainID := m.rangeCfg.ChainID

	for _, burn := range data.Burns {
		batch.Queue(fmt.Sprintf(`
			UPDATE %s.accounts
			SET general_balance = general_balance - $2
				WHERE address = $1;
		`, chainID),
			burn.Owner.String(),
			burn.Amount.ToBigInt().Uint64(),
		)
	}

	return nil
}

func (m *Main) queueEscrows(batch *storage.QueryBatch, data *storage.StakingData) error {
	chainID := m.rangeCfg.ChainID

	for _, escrow := range data.Escrows {
		switch e := escrow; {
		case e.Add != nil:
			owner := e.Add.Owner.String()
			escrower := e.Add.Escrow.String()
			amount := e.Add.Amount.ToBigInt().Uint64()
			newShares := e.Add.NewShares.ToBigInt().Uint64()
			batch.Queue(fmt.Sprintf(`
				UPDATE %s.accounts
				SET general_balance = general_balance - $2
					WHERE address = $1;
			`, chainID),
				owner,
				amount,
			)
			batch.Queue(fmt.Sprintf(`
				INSERT INTO %s.accounts (address, escrow_balance_active, escrow_total_shares_active)
					VALUES ($1, $2, $3)
				ON CONFLICT (address) DO
					UPDATE SET
						escrow_balance_active = %s.accounts.escrow_balance_active + $2,
						escrow_total_shares_active = %s.accounts.escrow_total_shares_active + $3;
			`, chainID, chainID, chainID),
				escrower,
				amount,
				newShares,
			)
			batch.Queue(fmt.Sprintf(`
				INSERT INTO %s.delegations (delegatee, delegator, shares)
					VALUES ($1, $2, $3)
				ON CONFLICT (delegatee, delegator) DO
					UPDATE SET shares = %s.delegations.shares + $3;
			`, chainID, chainID),
				owner,
				escrower,
				newShares,
			)
		case e.Take != nil:
			batch.Queue(fmt.Sprintf(`
				UPDATE %s.accounts
				SET escrow_balance_active = escrow_balance_active - $2
					WHERE address = $1;
			`, chainID),
				e.Take.Owner.String(),
				e.Take.Amount.ToBigInt().Uint64(),
			)
		case e.DebondingStart != nil:
			batch.Queue(fmt.Sprintf(`
				UPDATE %s.delegations
					SET shares = shares - $3
						WHERE delegatee = $1 AND delegator = $2;
			`, chainID),
				e.DebondingStart.Owner.String(),
				e.DebondingStart.Escrow.String(),
				e.DebondingStart.Amount.ToBigInt().Uint64(),
			)
			batch.Queue(fmt.Sprintf(`
				INSERT INTO %s.debonding_delegations (delegatee, delegator, shares, debond_end)
					VALUES ($1, $2, $3, $4);
			`, chainID),
				e.DebondingStart.Owner.String(),
				e.DebondingStart.Escrow.String(),
				e.DebondingStart.DebondingShares.ToBigInt().Uint64(),
				e.DebondingStart.DebondEndTime,
			)
		case e.Reclaim != nil:
			batch.Queue(fmt.Sprintf(`
				UPDATE %s.accounts
					SET general_balance = general_balance + $2
						WHERE address = $1;
			`, chainID),
				e.Reclaim.Owner.String(),
				e.Reclaim.Amount.ToBigInt().Uint64(),
			)

			batch.Queue(fmt.Sprintf(`
				UPDATE %s.accounts
					SET escrow_balance_active = escrow_balance_active - $2,
						escrow_total_shares_active = escrow_total_shares_active - $3
						WHERE address = $1;
			`, chainID),
				e.Reclaim.Escrow.String(),
				e.Reclaim.Amount.ToBigInt().Uint64(),
				e.Reclaim.Shares.ToBigInt().Uint64(),
			)
		}
	}

	return nil
}

func (m *Main) queueAllowanceChanges(batch *storage.QueryBatch, data *storage.StakingData) error {
	chainID := m.rangeCfg.ChainID

	for _, allowanceChange := range data.AllowanceChanges {
		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.allowances (owner, beneficiary, allowance)
				VALUES ($1, $2, $3)
			ON CONFLICT (owner, beneficiary) DO
				UPDATE SET allowance = excluded.allowance;
		`, chainID),
			allowanceChange.Owner.String(),
			allowanceChange.Beneficiary.String(),
			allowanceChange.Allowance.ToBigInt().Uint64(),
		)
	}

	return nil
}

// prepareSchedulerData adds scheduler data queries to the batch.
func (m *Main) prepareSchedulerData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.SchedulerData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.SchedulerData) error{
		m.queueValidatorUpdates,
		m.queueCommitteeUpdates,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueValidatorUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	chainID := m.rangeCfg.ChainID

	for _, validator := range data.Validators {
		batch.Queue(fmt.Sprintf(`
			UPDATE %s.nodes SET voting_power = $2
				WHERE id = $1;
		`, chainID),
			validator.ID,
			validator.VotingPower,
		)
	}

	return nil
}

func (m *Main) queueCommitteeUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	chainID := m.rangeCfg.ChainID

	batch.Queue(fmt.Sprintf(`
		TRUNCATE %s.committee_members;
	`, chainID))
	for namespace, committees := range data.Committees {
		runtime := namespace.String()
		for _, committee := range committees {
			kind := committee.String()
			validFor := int64(committee.ValidFor)
			for _, member := range committee.Members {
				batch.Queue(fmt.Sprintf(`
					INSERT INTO %s.committee_members (node, valid_for, runtime, kind, role)
						VALUES ($1, $2, $3, $4, $5);
				`, chainID),
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
func (m *Main) prepareGovernanceData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.GovernanceData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.GovernanceData) error{
		m.queueSubmissions,
		m.queueExecutions,
		m.queueFinalizations,
		m.queueVotes,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueSubmissions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	chainID := m.rangeCfg.ChainID

	for _, submission := range data.ProposalSubmissions {
		if submission.Content.Upgrade != nil {
			batch.Queue(fmt.Sprintf(`
				INSERT INTO %s.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, created_at, closes_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
			`, chainID),
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.ToBigInt().Uint64(),
				submission.Content.Upgrade.Handler,
				submission.Content.Upgrade.Target.ConsensusProtocol.String(),
				submission.Content.Upgrade.Target.RuntimeHostProtocol.String(),
				submission.Content.Upgrade.Target.RuntimeCommitteeProtocol.String(),
				submission.Content.Upgrade.Epoch,
				submission.CreatedAt,
				submission.ClosesAt,
			)
		} else if submission.Content.CancelUpgrade != nil {
			batch.Queue(fmt.Sprintf(`
				INSERT INTO %s.proposals (id, submitter, state, deposit, cancels, created_at, closes_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7);
			`, chainID),
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.ToBigInt().Uint64(),
				submission.Content.CancelUpgrade.ProposalID,
				submission.CreatedAt,
				submission.ClosesAt,
			)
		}
	}

	return nil
}

func (m *Main) queueExecutions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	chainID := m.rangeCfg.ChainID

	for _, execution := range data.ProposalExecutions {
		batch.Queue(fmt.Sprintf(`
			UPDATE %s.proposals
			SET executed = true
				WHERE id = $1;
		`, chainID),
			execution.ID,
		)
	}

	return nil
}

func (m *Main) queueFinalizations(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	chainID := m.rangeCfg.ChainID

	for _, finalization := range data.ProposalFinalizations {
		batch.Queue(fmt.Sprintf(`
			UPDATE %s.proposals
			SET state = $2
				WHERE id = $1;
		`, chainID),
			finalization.ID,
			finalization.State.String(),
		)
		batch.Queue(fmt.Sprintf(`
			UPDATE %s.proposals
			SET invalid_votes = $2
				WHERE id = $1;
		`, chainID),
			finalization.ID,
			finalization.InvalidVotes,
		)
	}

	return nil
}

func (m *Main) queueVotes(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	chainID := m.rangeCfg.ChainID

	for _, vote := range data.Votes {
		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.votes (proposal, voter, vote)
				VALUES ($1, $2, $3);
		`, chainID),
			vote.ID,
			vote.Submitter.String(),
			vote.Vote.String(),
		)
	}

	return nil
}

// extractEventData extracts the type of an event.
//
// TODO: Eliminate this if possible.
func extractEventData(event *results.Event) (backend analyzer.Backend, ty analyzer.Event, body []byte, err error) {
	switch e := event; {
	case e.Staking != nil:
		backend = analyzer.BackendStaking
		switch b := event.Staking; {
		case b.Transfer != nil:
			ty = analyzer.EventStakingTransfer
			body, err = json.Marshal(b.Transfer)
			return
		case b.Burn != nil:
			ty = analyzer.EventStakingBurn
			body, err = json.Marshal(b.Burn)
			return
		case b.Escrow != nil:
			switch t := b.Escrow; {
			case t.Add != nil:
				ty = analyzer.EventStakingAddEscrow
				body, err = json.Marshal(b.Escrow.Add)
				return
			case t.Take != nil:
				ty = analyzer.EventStakingTakeEscrow
				body, err = json.Marshal(b.Escrow.Take)
				return
			case t.DebondingStart != nil:
				ty = analyzer.EventStakingDebondingStart
				body, err = json.Marshal(b.Escrow.DebondingStart)
				return
			case t.Reclaim != nil:
				ty = analyzer.EventStakingReclaimEscrow
				body, err = json.Marshal(b.Escrow.Reclaim)
				return
			}
		case b.AllowanceChange != nil:
			ty = analyzer.EventStakingAllowanceChange
			body, err = json.Marshal(b.AllowanceChange)
			return
		}
	case e.Registry != nil:
		backend = analyzer.BackendRegistry
		switch b := event.Registry; {
		case b.RuntimeEvent != nil:
			ty = analyzer.EventRegistryRuntimeRegistration
			body, err = json.Marshal(b.RuntimeEvent)
			return
		case b.EntityEvent != nil:
			ty = analyzer.EventRegistryEntityRegistration
			body, err = json.Marshal(b.EntityEvent)
			return
		case b.NodeEvent != nil:
			ty = analyzer.EventRegistryNodeRegistration
			body, err = json.Marshal(b.NodeEvent)
			return
		case b.NodeUnfrozenEvent != nil:
			ty = analyzer.EventRegistryNodeUnfrozenEvent
			body, err = json.Marshal(b.NodeUnfrozenEvent)
			return
		}
	case e.RootHash != nil:
		backend = analyzer.BackendRoothash
		switch b := event.RootHash; {
		case b.ExecutorCommitted != nil:
			ty = analyzer.EventRoothashExecutorCommittedEvent
			body, err = json.Marshal(event.RootHash.ExecutorCommitted)
			return
		case b.ExecutionDiscrepancyDetected != nil:
			ty = analyzer.EventRoothashDiscrepancyDetectedEvent
			body, err = json.Marshal(event.RootHash.ExecutionDiscrepancyDetected)
			return
		case b.Finalized != nil:
			ty = analyzer.EventRoothashFinalizedEvent
			body, err = json.Marshal(event.RootHash.Finalized)
			return
		}
	case e.Governance != nil:
		backend = analyzer.BackendGovernance
		switch b := event.Governance; {
		case b.ProposalSubmitted != nil:
			ty = analyzer.EventGovernanceProposalSubmitted
			body, err = json.Marshal(event.Governance.ProposalSubmitted)
			return
		case b.ProposalExecuted != nil:
			ty = analyzer.EventGovernanceProposalExecuted
			body, err = json.Marshal(event.Governance.ProposalExecuted)
			return
		case b.ProposalFinalized != nil:
			ty = analyzer.EventGovernanceProposalExecuted
			body, err = json.Marshal(event.Governance.ProposalFinalized)
			return
		case b.Vote != nil:
			ty = analyzer.EventGovernanceVote
			body, err = json.Marshal(event.Governance.Vote)
			return
		}
	}

	return analyzer.BackendUnknown, analyzer.EventUnknown, []byte{}, errors.New("unknown event type")
}
