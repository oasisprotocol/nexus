package types

import "fmt"

const (
	Erc20Transfer = "erc20.transfer"
	Erc20Approval = "erc20.approval"
)

type Address string

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
	case LayerConsensus, LayerEmerald:
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
