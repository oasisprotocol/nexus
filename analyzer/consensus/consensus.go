// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"golang.org/x/sync/errgroup"

	"github.com/oasislabs/oasis-block-indexer/go/analyzer"
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	chainID = "oasis_3"

	analyzerName = "consensus_main_damask"
)

var (
	// ErrOutOfRange is returned if the current block does not fall within tge
	// analyzer's analysis range.
	ErrOutOfRange = errors.New("range not found. no data source available.")

	// ErrLatestBlockNotFound is returned if the analyzer has not indexed any
	// blocks yet. This indicates to begin from the start of its range.
	ErrLatestBlockNotFound = errors.New("latest block not found")
)

// ConsensusMain is the main Analyzer for the consensus layer.
type ConsensusMain struct {
	rangeCfg analyzer.RangeConfig
	target   storage.TargetStorage
	logger   *log.Logger
}

// NewConsensusMain returns a new analyzer for the consensus layer.
func NewConsensusMain(target storage.TargetStorage, logger *log.Logger) *ConsensusMain {
	return &ConsensusMain{
		target: target,
		logger: logger.With("analyzer", analyzerName),
	}
}

// SetRange adds configuration for the range of blocks to process to
// this analyzer. It is intended to be called before Start.
func (c *ConsensusMain) SetRange(cfg analyzer.RangeConfig) {
	c.rangeCfg = cfg
}

// Start starts the main consensus analyzer.
func (c *ConsensusMain) Start() {
	ctx := context.Background()

	// Get block to be indexed.
	var height int64

	latest, err := c.latestBlock(ctx)
	if err != nil {
		if err != pgx.ErrNoRows {
			c.logger.Error("last block height not found",
				"err", err.Error(),
			)
			return
		}
		height = c.rangeCfg.From
	} else {
		height = latest + 1
	}

	for {
		if err := c.processBlock(ctx, height); err != nil {
			if err == ErrOutOfRange {
				c.logger.Info("no data source available at this height",
					"height", height,
				)
				return
			}

			c.logger.Error("error processing block",
				"err", err.Error(),
			)
			continue
		}

		height++
	}
}

// Name returns the name of the ConsensusMain.
func (c *ConsensusMain) Name() string {
	return analyzerName
}

// source returns the source storage for the provided block height.
func (c *ConsensusMain) source(height int64) (storage.SourceStorage, error) {
	r := c.rangeCfg
	if height >= r.From && (height <= r.To || r.To == 0) {
		return r.Source, nil
	}

	return nil, ErrOutOfRange
}

// latestBlock returns the latest block processed by the consensus analyzer.
func (c *ConsensusMain) latestBlock(ctx context.Context) (int64, error) {
	row, err := c.target.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT height FROM %s.processed_blocks
				WHERE analyzer = $1
				ORDER BY height DESC
				LIMIT 1
		`, chainID),
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
func (c *ConsensusMain) processBlock(ctx context.Context, height int64) error {
	c.logger.Info("processing block",
		"height", height,
	)

	group, groupCtx := errgroup.WithContext(ctx)

	// Prepare and perform updates.
	batch := &storage.QueryBatch{}

	type prepareFunc = func(context.Context, int64, *storage.QueryBatch) error
	for _, f := range []prepareFunc{
		c.prepareBlockData,
		c.prepareRegistryData,
		c.prepareSchedulerData,
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

	return c.target.SendBatch(ctx, batch)
}

// prepareBlockData adds block data queries to the batch.
func (c *ConsensusMain) prepareBlockData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := c.source(height)
	if err != nil {
		return err
	}

	data, err := source.BlockData(ctx, height)
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

func (c *ConsensusMain) queueBlockInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
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

func (c *ConsensusMain) queueTransactionInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
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
			INSERT INTO %s.transactions (block, txn_hash, txn_index, nonce, fee_amount, max_gas, method, body, module, code, message)
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

func (c *ConsensusMain) queueEventInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
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

// prepareRegistryData adds registry data queries to the batch
func (c *ConsensusMain) prepareRegistryData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := c.source(height)
	if err != nil {
		return err
	}

	data, err := source.RegistryData(ctx, height)

	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.RegistryData) error{
		c.queueRuntimeRegistrations,
		c.queueEntityEvents,
		c.queueNodeRegistrations,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConsensusMain) queueRuntimeRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
	for _, runtimeEvent := range data.RuntimeEvents {
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
			runtimeEvent.Runtime.KeyManager,
		)
	}

	return nil
}

func (c *ConsensusMain) queueEntityEvents(batch *storage.QueryBatch, data *storage.RegistryData) error {
	for _, entityEvent := range data.EntityEvents {
		var nodes []string
		for _, node := range entityEvent.Entity.Nodes {
			nodes = append(nodes, node.String())
		}
		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.entities (id, address) VALUES ($1, $2, $3)
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

func (c *ConsensusMain) queueNodeRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
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
			fmt.Sprintf(`{"%s"}`, strings.Join(tlsAddresses, `','`)),
			nodeEvent.Node.P2P.ID.String(),
			fmt.Sprintf(`{"%s"}`, strings.Join(p2pAddresses, `','`)),
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

// prepareSchedulerData adds scheduler data queries to the batch.
func (c *ConsensusMain) prepareSchedulerData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := c.source(height)
	if err != nil {
		return err
	}

	data, err := source.SchedulerData(ctx, height)

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

func (c *ConsensusMain) queueValidatorUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
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

func (c *ConsensusMain) queueCommitteeUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
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

// extractEventData extracts the type of an event.
//
// TODO: Eliminate this if possible.
func extractEventData(event *results.Event) (backend analyzer.Backend, ty analyzer.Event, body []byte, err error) {
	if event.Staking != nil {
		backend = analyzer.BackendStaking
		if event.Staking.Transfer != nil {
			ty = analyzer.EventStakingTransfer
			body, err = json.Marshal(event.Staking.Transfer)
			return
		} else if event.Staking.Burn != nil {
			ty = analyzer.EventStakingTransfer
			body, err = json.Marshal(event.Staking.Burn)
			return
		} else if event.Staking.Escrow != nil {
			if event.Staking.Escrow.Add != nil {
				ty = analyzer.EventStakingAddEscrow
				body, err = json.Marshal(event.Staking.Escrow.Add)
				return
			} else if event.Staking.Escrow.Take != nil {
				ty = analyzer.EventStakingTakeEscrow
				body, err = json.Marshal(event.Staking.Escrow.Take)
				return
			} else if event.Staking.Escrow.DebondingStart != nil {
				ty = analyzer.EventStakingDebondingStart
				body, err = json.Marshal(event.Staking.Escrow.DebondingStart)
				return
			} else if event.Staking.Escrow.Reclaim != nil {
				ty = analyzer.EventStakingReclaimEscrow
				body, err = json.Marshal(event.Staking.Escrow.Reclaim)
				return
			}
		} else if event.Staking.AllowanceChange != nil {
			ty = analyzer.EventStakingAllowanceChange
			body, err = json.Marshal(event.Staking.AllowanceChange)
			return
		}
	} else if event.Registry != nil {
		backend = analyzer.BackendRegistry
		if event.Registry.RuntimeEvent != nil {
			ty = analyzer.EventRegistryRuntimeRegistration
			body, err = json.Marshal(event.Registry.RuntimeEvent)
			return
		} else if event.Registry.EntityEvent != nil {
			ty = analyzer.EventRegistryEntityRegistration
			body, err = json.Marshal(event.Registry.EntityEvent)
			return
		} else if event.Registry.NodeEvent != nil {
			ty = analyzer.EventRegistryNodeRegistration
			body, err = json.Marshal(event.Registry.NodeEvent)
			return
		} else if event.Registry.NodeUnfrozenEvent != nil {
			ty = analyzer.EventRegistryNodeUnfrozenEvent
			body, err = json.Marshal(event.Registry.NodeUnfrozenEvent)
			return
		}
	} else if event.RootHash != nil {
		backend = analyzer.BackendRoothash
		if event.RootHash.ExecutorCommitted != nil {
			ty = analyzer.EventRoothashExecutorCommittedEvent
			body, err = json.Marshal(event.RootHash.ExecutorCommitted)
			return
		} else if event.RootHash.ExecutionDiscrepancyDetected != nil {
			ty = analyzer.EventRoothashDiscrepancyDetectedEvent
			body, err = json.Marshal(event.RootHash.ExecutionDiscrepancyDetected)
			return
		} else if event.RootHash.Finalized != nil {
			ty = analyzer.EventRoothashFinalizedEvent
			body, err = json.Marshal(event.RootHash.Finalized)
			return
		}
	} else if event.Governance != nil {
		backend = analyzer.BackendGovernance
		if event.Governance.ProposalSubmitted != nil {
			ty = analyzer.EventGovernanceProposalSubmitted
			body, err = json.Marshal(event.Governance.ProposalSubmitted)
			return
		} else if event.Governance.ProposalExecuted != nil {
			ty = analyzer.EventGovernanceProposalExecuted
			body, err = json.Marshal(event.Governance.ProposalExecuted)
			return
		} else if event.Governance.ProposalFinalized != nil {
			ty = analyzer.EventGovernanceProposalExecuted
			body, err = json.Marshal(event.Governance.ProposalFinalized)
			return
		} else if event.Governance.Vote != nil {
			ty = analyzer.EventGovernanceVote
			body, err = json.Marshal(event.Governance.Vote)
			return
		}
	}

	return analyzer.BackendUnknown, analyzer.EventUnknown, []byte{}, errors.New("unknown event type")
}
