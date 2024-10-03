// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	jsoniter "github.com/json-iterator/go" // Supports marshalling map[interface{}]interface{}, unlike encoding/json

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/entity"

	beaconCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/beacon/api"
	governanceCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/governance/api"
	keymanagerCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/keymanager/api"
	registryCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/registry/api"
	roothashCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/roothash/api"
	stakingCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/staking/api"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/common/node"
	governance "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	keymanager "github.com/oasisprotocol/nexus/coreapi/v22.2.11/keymanager/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"

	beaconEden "github.com/oasisprotocol/nexus/coreapi/v24.0/beacon/api"
	consensusEden "github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api"
	governanceEden "github.com/oasisprotocol/nexus/coreapi/v24.0/governance/api"
	keymanagerChurpEden "github.com/oasisprotocol/nexus/coreapi/v24.0/keymanager/churp"
	keymanagerSecretsEden "github.com/oasisprotocol/nexus/coreapi/v24.0/keymanager/secrets"
	registryEden "github.com/oasisprotocol/nexus/coreapi/v24.0/registry/api"
	roothashEden "github.com/oasisprotocol/nexus/coreapi/v24.0/roothash/api"
	stakingEden "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
	vaultEden "github.com/oasisprotocol/nexus/coreapi/v24.0/vault/api"
)

var bodyTypeForTxMethodCobalt = map[string]interface{}{
	"beacon.PVSSCommit":                pvssCommitCobalt{},
	"beacon.PVSSReveal":                pvssRevealCobalt{},
	"governance.SubmitProposal":        governanceCobalt.ProposalContent{},
	"governance.CastVote":              governanceCobalt.ProposalVote{},
	"keymanager.UpdatePolicy":          keymanagerCobalt.SignedPolicySGX{},
	"registry.RegisterEntity":          entity.SignedEntity{},  // We didn't vendor the entity Cobalt package because it's identical
	"registry.RegisterNode":            node.MultiSignedNode{}, // We didn't vendor the node Cobalt package because it's identical
	"registry.UnfreezeNode":            registryCobalt.UnfreezeNode{},
	"registry.RegisterRuntime":         registryCobalt.Runtime{},
	"roothash.ExecutorCommit":          roothashCobalt.ExecutorCommit{},
	"roothash.ExecutorProposerTimeout": roothashCobalt.ExecutorProposerTimeoutRequest{},
	"roothash.Evidence":                roothashCobalt.Evidence{},
	"staking.Transfer":                 stakingCobalt.Transfer{},
	"staking.Burn":                     stakingCobalt.Burn{},
	"staking.AddEscrow":                stakingCobalt.Escrow{},
	"staking.ReclaimEscrow":            stakingCobalt.ReclaimEscrow{},
	"staking.AmendCommissionSchedule":  stakingCobalt.AmendCommissionSchedule{},
	"staking.Allow":                    stakingCobalt.Allow{},
	"staking.Withdraw":                 stakingCobalt.Withdraw{},
}

var bodyTypeForTxMethodDamask = map[string]interface{}{
	"beacon.VRFProve":                  beacon.VRFProve{},
	"governance.SubmitProposal":        governance.ProposalContent{},
	"governance.CastVote":              governance.ProposalVote{},
	"keymanager.UpdatePolicy":          keymanager.SignedPolicySGX{},
	"registry.RegisterEntity":          entity.SignedEntity{}, // We didn't vendor the entityDamask package because it's identical
	"registry.DeregisterEntity":        registry.DeregisterEntity{},
	"registry.RegisterNode":            node.MultiSignedNode{}, // We didn't vendor the node Damask package because it's identical
	"registry.UnfreezeNode":            registry.UnfreezeNode{},
	"registry.RegisterRuntime":         registry.Runtime{},
	"registry.ProveFreshness":          registry.Runtime{}, // not sure when the proveFreshness tx body changed; so we keep the old one here.
	"roothash.ExecutorCommit":          roothash.ExecutorCommit{},
	"roothash.ExecutorProposerTimeout": roothash.ExecutorProposerTimeoutRequest{},
	"roothash.Evidence":                roothash.Evidence{},
	"roothash.SubmitMsg":               roothash.SubmitMsg{},
	"staking.Transfer":                 staking.Transfer{},
	"staking.Burn":                     staking.Burn{},
	"staking.AddEscrow":                staking.Escrow{},
	"staking.ReclaimEscrow":            staking.ReclaimEscrow{},
	"staking.AmendCommissionSchedule":  staking.AmendCommissionSchedule{},
	"staking.Allow":                    staking.Allow{},
	"staking.Withdraw":                 staking.Withdraw{},
}

var bodyTypeForTxMethodEden = map[string]interface{}{
	"beacon.VRFProve":                   beaconEden.VRFProve{},
	"consensus.Meta":                    consensusEden.BlockMetadata{},
	"governance.SubmitProposal":         governanceEden.ProposalContent{},
	"governance.CastVote":               governanceEden.ProposalVote{},
	"keymanager.PublishMasterSecret":    keymanagerSecretsEden.SignedEncryptedMasterSecret{},
	"keymanager.PublishEphemeralSecret": keymanagerSecretsEden.SignedEncryptedEphemeralSecret{},
	"keymanager.UpdatePolicy":           keymanagerSecretsEden.SignedPolicySGX{},
	"registry.RegisterEntity":           entity.SignedEntity{},
	"registry.DeregisterEntity":         registryEden.DeregisterEntity{},
	"registry.RegisterNode":             node.MultiSignedNode{},
	"registry.UnfreezeNode":             registryEden.UnfreezeNode{},
	"registry.RegisterRuntime":          registryEden.Runtime{},
	"registry.ProveFreshness":           freshnessProofEden{},
	"roothash.ExecutorCommit":           roothashEden.ExecutorCommit{},
	"roothash.Evidence":                 roothashEden.Evidence{},
	"roothash.SubmitMsg":                roothashEden.SubmitMsg{},
	"staking.Transfer":                  stakingEden.Transfer{},
	"staking.Burn":                      stakingEden.Burn{},
	"staking.AddEscrow":                 stakingEden.Escrow{},
	"staking.ReclaimEscrow":             stakingEden.ReclaimEscrow{},
	"staking.AmendCommissionSchedule":   stakingEden.AmendCommissionSchedule{},
	"staking.Allow":                     stakingEden.Allow{},
	"staking.Withdraw":                  stakingEden.Withdraw{},
	// Added in v24.0.
	"keymanager/churp.Apply":   keymanagerChurpEden.ApplicationRequest{},
	"keymanager/churp.Confirm": keymanagerChurpEden.ConfirmationRequest{},
	"keymanager/churp.Create":  keymanagerChurpEden.CreateRequest{},
	"keymanager/churp.Update":  keymanagerChurpEden.UpdateRequest{},
	"vault.AuthorizeAction":    vaultEden.AuthorizeAction{},
	"vault.CancelAction":       vaultEden.CancelAction{},
	"vault.Create":             vaultEden.Create{},
}

type pvssCommitCobalt struct {
	inner beaconCobalt.PVSSCommit
}

func (pc *pvssCommitCobalt) UnmarshalCBOR(data []byte) error {
	var c beaconCobalt.PVSSCommit
	if err := cbor.Unmarshal(data, &c); err != nil {
		return err
	}
	pc.inner = c
	return nil
}

func (pc *pvssCommitCobalt) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(pc.inner)
}

type pvssRevealCobalt struct {
	inner beaconCobalt.PVSSReveal
}

func (pc *pvssRevealCobalt) UnmarshalCBOR(data []byte) error {
	var c beaconCobalt.PVSSReveal
	if err := cbor.Unmarshal(data, &c); err != nil {
		return err
	}
	pc.inner = c
	return nil
}

func (pc *pvssRevealCobalt) MarshalJSON() ([]byte, error) {
	return jsoniter.Marshal(pc.inner)
}

type freshnessProofEden struct {
	Blob []byte `json:"blob"`
}

func (f *freshnessProofEden) UnmarshalCBOR(data []byte) error {
	var blob []byte
	if err := cbor.Unmarshal(data, &blob); err != nil {
		return err
	}
	f.Blob = blob
	return nil
}
