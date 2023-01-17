package types

var StringToEvent map[string]ConsensusEventType = map[string]ConsensusEventType{
	"staking.transfer":                        ConsensusEventTypeStakingTransfer,
	"staking.burn":                            ConsensusEventTypeStakingBurn,
	"staking.escrow.add":                      ConsensusEventTypeStakingEscrowAdd,
	"staking.escrow.take":                     ConsensusEventTypeStakingEscrowTake,
	"staking.escrow.debonding_start":          ConsensusEventTypeStakingEscrowDebondingStart,
	"staking.escrow.reclaim":                  ConsensusEventTypeStakingEscrowReclaim,
	"staking.allowance_change":                ConsensusEventTypeStakingAllowanceChange,
	"registry.runtime":                        ConsensusEventTypeRegistryRuntime,
	"registry.entity":                         ConsensusEventTypeRegistryEntity,
	"registry.node":                           ConsensusEventTypeRegistryNode,
	"registry.node_unfrozen":                  ConsensusEventTypeRegistryNodeUnfrozen,
	"roothash.executor_committed":             ConsensusEventTypeRoothashExecutorCommitted,
	"roothash.execution_discrepancy_detected": ConsensusEventTypeRoothashExecutionDiscrepancy,
	"roothash.finalized":                      ConsensusEventTypeRoothashFinalized,
	"governance.proposal_submitted":           ConsensusEventTypeGovernanceProposalSubmitted,
	"governance.proposal_executed":            ConsensusEventTypeGovernanceProposalExecuted,
	"governance.proposal_finalized":           ConsensusEventTypeGovernanceProposalFinalized,
	"governance.vote":                         ConsensusEventTypeGovernanceVote,
}
