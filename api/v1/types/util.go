package types

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
