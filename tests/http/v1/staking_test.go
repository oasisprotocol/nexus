package v1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/common"
	storage "github.com/oasisprotocol/nexus/storage/client"
	"github.com/oasisprotocol/nexus/tests"
)

var stakingEndHeight int64 = 8054649

func makeTestAccounts() []storage.Account {
	return []storage.Account{
		{
			Address:   "oasis1qp28vcurlx03y9exedzd9kfp7u2p0f0nvvv7h5wv",
			Nonce:     1,
			Available: common.NewBigInt(0),
			Escrow:    common.NewBigInt(0),
			Debonding: common.NewBigInt(0),
		},
		{
			Address:   "oasis1qrj5x6twyjg0lxkz9kv0y9tyhzpxwq9u6v6sgje2",
			Nonce:     0,
			Available: common.NewBigInt(5600000000),
			Escrow:    common.NewBigInt(0),
			Debonding: common.NewBigInt(0),
		},
	}
}

func TestListAccounts(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	<-tests.After(stakingEndHeight)

	var list storage.AccountList
	err := tests.GetFrom("/consensus/accounts?minAvailable=1000000000000000000", &list)
	require.NoError(t, err)
	require.Equal(t, 1, len(list.Accounts))

	// The big kahuna (Binance Staking).
	require.Equal(t, "oasis1qpg2xuz46g53737343r20yxeddhlvc2ldqsjh70p", list.Accounts[0].Address)
	require.Greater(t, list.Accounts[0].Available, common.NewBigInt(1000000000000000000))
}

func TestGetAccount(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testAccounts := makeTestAccounts()
	<-tests.After(stakingEndHeight)

	for _, testAccount := range testAccounts {
		var account storage.Account
		err := tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", testAccount.Address), &account)
		require.NoError(t, err)
		require.Equal(t, testAccount, account)
	}
}
