package main

import (
	"context"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi/history"
	"github.com/oasisprotocol/nexus/storage/postgres"

	staking "github.com/oasisprotocol/nexus/coreapi/v23.0/staking/api"
)

// Returns number of shares delegated by `delegator` to `delegatee` at `height`.
func delegationViaNodeApi(ctx context.Context, nodeApi *history.HistoryConsensusApiLite, height int64, delegator, delegatee staking.Address) common.BigInt {
	api, err := nodeApi.APIForHeight(4743433)
	if err != nil {
		panic(err)
	}
	grpcConn := api.GrpcConn()

	m := map[staking.Address]*staking.Delegation{}
	// DelegationsTo is faster than DelegationsFor on old archive nodes.
	if err := grpcConn.Invoke(ctx, "/oasis-core.Staking/DelegationsTo", staking.OwnerQuery{Height: height, Owner: delegatee}, &m); err != nil {
		panic(err)
	}

	delegation, ok := m[delegator]
	if !ok {
		return common.BigInt{}
	}
	return common.BigIntFromQuantity(delegation.Shares)
}

// Returns number of shares delegated by `delegator` to `delegatee` at `height`, according to dead-reckoned DB values.
func delegationViaDeadReckon(ctx context.Context, db *postgres.Client, height int64, delegator, delegatee staking.Address) common.BigInt {
	// We cannot simply "SELECT shares FROM chain.delegations WHERE delegatee=$1 AND delegator=$2" because that only gives the most recent value.
	// Simulate dead-reckoning instead.
	shares := common.BigInt{}
	if err := db.QueryRow(ctx, `
		WITH candidate_events AS (
			SELECT * FROM chain.events WHERE
			ARRAY[$1, $2] <@ related_accounts AND  -- implied by the next condition anyway, but this speeds up the query
			body->>'owner'=$1 and body->>'escrow'=$2 AND
			tx_block <= $3
		)
		SELECT (
			  (SELECT COALESCE(SUM((body->>'new_shares'   )::NUMERIC), 0) FROM candidate_events WHERE type='staking.escrow.add')
			- (SELECT COALESCE(SUM((body->>'active_shares')::NUMERIC), 0) FROM candidate_events WHERE type='staking.escrow.debonding_start')
		)`,
		delegatee, delegator, height,
	).Scan(&shares); err != nil {
		panic(err)
	}
	return shares
}
