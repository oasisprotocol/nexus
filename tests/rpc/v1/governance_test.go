package v1

import (
	"fmt"
	"testing"

	v1 "github.com/oasislabs/oasis-indexer/api/v1"
	"github.com/oasislabs/oasis-indexer/tests"
	"github.com/stretchr/testify/require"
)

func makeTestProposals(t *testing.T) []v1.Proposal {
	p1Handler := "consensus-params-update-2021-08"
	p1Epoch := uint64(8049)
	p1ConsensusTarget := "4.0.0"
	p1RuntimeHostTarget := "3.0.0"
	p1RuntimeCommitteeTarget := "2.0.0"
	p1Target := v1.Target{
		ConsensusProtocol:        &p1ConsensusTarget,
		RuntimeHostProtocol:      &p1RuntimeHostTarget,
		RuntimeCommitteeProtocol: &p1RuntimeCommitteeTarget,
	}

	p2Handler := "mainnet-upgrade-2022-04-11"
	p2Epoch := uint64(13402)
	p2ConsensusTarget := "5.0.0"
	p2RuntimeHostTarget := "5.0.0"
	p2RuntimeCommitteeTarget := "4.0.0"
	p2Target := v1.Target{
		ConsensusProtocol:        &p2ConsensusTarget,
		RuntimeHostProtocol:      &p2RuntimeHostTarget,
		RuntimeCommitteeProtocol: &p2RuntimeCommitteeTarget,
	}
	return []v1.Proposal{
		{
			ID:           1,
			Submitter:    "oasis1qpydpeyjrneq20kh2jz2809lew6d9p64yymutlee",
			State:        "passed",
			Deposit:      10000000000000,
			Handler:      &p1Handler,
			Target:       p1Target,
			Epoch:        &p1Epoch,
			CreatedAt:    7708,
			ClosesAt:     7876,
			InvalidVotes: 2,
		},
		{
			ID:           2,
			Submitter:    "oasis1qpydpeyjrneq20kh2jz2809lew6d9p64yymutlee",
			State:        "passed",
			Deposit:      10000000000000,
			Handler:      &p2Handler,
			Target:       p2Target,
			Epoch:        &p2Epoch,
			CreatedAt:    12984,
			ClosesAt:     13152,
			InvalidVotes: 1,
		},
	}
}

func TestListProposals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testProposals := makeTestProposals(t)
	<-tests.After(tests.GenesisHeight)

	var list v1.ProposalList
	tests.GetFrom("/consensus/proposals", &list)
	require.Equal(t, 2, len(list.Proposals))

	for i, proposal := range list.Proposals {
		require.Equal(t, testProposals[i], proposal)
	}
}

func TestGetProposal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testProposals := makeTestProposals(t)
	<-tests.After(tests.GenesisHeight)

	for i, testProposal := range testProposals {
		var proposal v1.Proposal
		tests.GetFrom(fmt.Sprintf("/consensus/proposals/%d", i+1), &proposal)
		require.Equal(t, testProposal, proposal)
	}
}

func TestGetProposalVotes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	expectedVotes := []int{77, 90}
	<-tests.After(tests.GenesisHeight)

	for i, expected := range expectedVotes {
		var votes v1.ProposalVotes
		tests.GetFrom(fmt.Sprintf("/consensus/proposals/%d/votes", i+1), &votes)
		require.Equal(t, uint64(i+1), votes.ProposalID)
		require.Equal(t, expected, len(votes.Votes))
	}
}
