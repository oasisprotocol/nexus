package v1

import (
	"fmt"
	"testing"

	v1 "github.com/oasislabs/oasis-indexer/api/v1"
	"github.com/oasislabs/oasis-indexer/tests"
	"github.com/stretchr/testify/require"
)

var (
	stakingEndHeight int64 = 8054649
)

func makeTestAccounts(t *testing.T) []v1.Account {
	return []v1.Account{
		{
			Address:   "oasis1qp28vcurlx03y9exedzd9kfp7u2p0f0nvvv7h5wv",
			Nonce:     1,
			Available: 0,
			Escrow:    0,
			Debonding: 0,
			Total:     0,
		},
		{
			Address:   "oasis1qrj5x6twyjg0lxkz9kv0y9tyhzpxwq9u6v6sgje2",
			Nonce:     0,
			Available: 56900000000,
			Escrow:    0,
			Debonding: 0,
			Total:     56900000000,
		},
	}
}

func TestListAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	<-tests.After(stakingEndHeight)

	var list v1.AccountList
	tests.GetFrom("/consensus/accounts?minAvailable=1000000000000000000", &list)
	require.Equal(t, 1, len(list.Accounts))

	// The big kahuna (Binance Staking).
	require.Equal(t, "oasis1qpg2xuz46g53737343r20yxeddhlvc2ldqsjh70p", list.Accounts[0].Address)
}

func TestGetAccount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testAccounts := makeTestAccounts(t)
	<-tests.After(stakingEndHeight)

	for _, testAccount := range testAccounts {
		var account v1.Account
		tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", testAccount.Address), &account)
		require.Equal(t, testAccount, account)
	}
}
