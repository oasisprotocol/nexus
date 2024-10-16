package v1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/common"
	storage "github.com/oasisprotocol/nexus/storage/client"
	"github.com/oasisprotocol/nexus/tests"
)

func makeTestProposals() []storage.Proposal {
	p1Handler := "consensus-params-update-2021-08"
	p1Epoch := uint64(8049)
	p1ConsensusTarget := "4.0.0"
	p1RuntimeHostTarget := "3.0.0"
	p1RuntimeCommitteeTarget := "2.0.0"
	p1Target := storage.ProposalTarget{
		ConsensusProtocol:        &p1ConsensusTarget,
		RuntimeHostProtocol:      &p1RuntimeHostTarget,
		RuntimeCommitteeProtocol: &p1RuntimeCommitteeTarget,
	}

	p2Handler := "mainnet-upgrade-2022-04-11"
	p2Epoch := uint64(13402)
	p2ConsensusTarget := "5.0.0"
	p2RuntimeHostTarget := "5.0.0"
	p2RuntimeCommitteeTarget := "4.0.0"
	p2Target := storage.ProposalTarget{
		ConsensusProtocol:        &p2ConsensusTarget,
		RuntimeHostProtocol:      &p2RuntimeHostTarget,
		RuntimeCommitteeProtocol: &p2RuntimeCommitteeTarget,
	}
	return []storage.Proposal{
		{
			ID:           1,
			Submitter:    "oasis1qpydpeyjrneq20kh2jz2809lew6d9p64yymutlee",
			State:        "passed",
			Deposit:      common.NewBigInt(10000000000000),
			Handler:      &p1Handler,
			Target:       &p1Target,
			Epoch:        &p1Epoch,
			CreatedAt:    7708,
			ClosesAt:     7876,
			InvalidVotes: common.NewBigInt(2),
		},
		{
			ID:           2,
			Submitter:    "oasis1qpydpeyjrneq20kh2jz2809lew6d9p64yymutlee",
			State:        "passed",
			Deposit:      common.NewBigInt(10000000000000),
			Handler:      &p2Handler,
			Target:       &p2Target,
			Epoch:        &p2Epoch,
			CreatedAt:    12984,
			ClosesAt:     13152,
			InvalidVotes: common.NewBigInt(1),
		},
	}
}

func TestListProposals(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testProposals := makeTestProposals()
	<-tests.After(tests.GenesisHeight)

	var list storage.ProposalList
	err := tests.GetFrom("/consensus/proposals", &list)
	require.NoError(t, err)
	require.Equal(t, 2, len(list.Proposals))

	for i, proposal := range list.Proposals {
		require.Equal(t, testProposals[i], proposal)
	}
}

func TestGetProposal(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testProposals := makeTestProposals()
	<-tests.After(tests.GenesisHeight)

	for i, testProposal := range testProposals {
		var proposal storage.Proposal
		err := tests.GetFrom(fmt.Sprintf("/consensus/proposals/%d", i+1), &proposal)
		require.NoError(t, err)
		require.Equal(t, testProposal, proposal)
	}
}

func TestGetProposalVotes(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	expectedVotes := []int{77, 90}
	<-tests.After(tests.GenesisHeight)

	for i, expected := range expectedVotes {
		var votes storage.ProposalVotes
		err := tests.GetFrom(fmt.Sprintf("/consensus/proposals/%d/votes", i+1), &votes)
		require.NoError(t, err)
		require.Equal(t, uint64(i+1), votes.ProposalID)
		require.Equal(t, expected, len(votes.Votes))
	}
}
