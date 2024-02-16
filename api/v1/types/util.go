package types

import "fmt"

// Hardcoded event names emitted by ERC-20 contracts.
// TODO: Query the ABI of the contract for the event names, remove these.
const (
	Erc20Transfer = "Transfer"
	Erc20Approval = "Approval"
)

func (c ConsensusEventType) IsValid() bool {
	switch c {
	case ConsensusEventTypeStakingTransfer,
		ConsensusEventTypeStakingBurn,
		ConsensusEventTypeStakingEscrowAdd,
		ConsensusEventTypeStakingEscrowTake,
		ConsensusEventTypeStakingEscrowDebondingStart,
		ConsensusEventTypeStakingEscrowReclaim,
		ConsensusEventTypeStakingAllowanceChange,
		ConsensusEventTypeRegistryRuntime,
		ConsensusEventTypeRegistryEntity,
		ConsensusEventTypeRegistryNode,
		ConsensusEventTypeRegistryNodeUnfrozen,
		ConsensusEventTypeRoothashExecutorCommitted,
		ConsensusEventTypeRoothashExecutionDiscrepancy,
		ConsensusEventTypeRoothashFinalized,
		ConsensusEventTypeGovernanceProposalSubmitted,
		ConsensusEventTypeGovernanceProposalExecuted,
		ConsensusEventTypeGovernanceProposalFinalized,
		ConsensusEventTypeGovernanceVote:
		return true
	default:
		return false
	}
}

func (c Layer) IsValid() bool {
	switch c {
	case LayerConsensus, LayerCipher, LayerEmerald, LayerSapphire:
		return true
	default:
		return false
	}
}

func (c Runtime) IsValid() bool {
	switch c {
	case RuntimeCipher, RuntimeEmerald, RuntimeSapphire:
		return true
	default:
		return false
	}
}

// Validate validates the parameters.
func (p *GetLayerStatsActiveAccountsParams) Validate() error {
	if p.WindowStepSeconds == nil {
		return nil
	}
	if *p.WindowStepSeconds == 300 || *p.WindowStepSeconds == 86400 {
		return nil
	}
	return &InvalidParamFormatError{ParamName: "window_step_seconds", Err: fmt.Errorf("invalid value: %d", *p.WindowStepSeconds)}
}
