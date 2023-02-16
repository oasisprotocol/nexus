package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

// Client is an implementation of RuntimeStorage that supports EVMSimulateCall
// by calling over Ethereum JSON-RPC.
type Client struct {
	client *ethclient.Client
}

var _ storage.RuntimeSourceStorage = (*Client)(nil)

func (ec *Client) AllData(ctx context.Context, round uint64) (*storage.RuntimeAllData, error) {
	panic("not implemented")
}

func (ec *Client) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error) {
	toECAddr := ethCommon.BytesToAddress(address)
	gasPriceBI := &big.Int{}
	gasPriceBI.SetBytes(gasPrice)
	valueBI := &big.Int{}
	valueBI.SetBytes(value)
	out, err := ec.client.CallContract(ctx, ethereum.CallMsg{
		From:     ethCommon.BytesToAddress(caller),
		To:       &toECAddr,
		Gas:      gasLimit,
		GasPrice: gasPriceBI,
		Value:    valueBI,
		Data:     data,
	}, big.NewInt(int64(round)))
	if err != nil {
		return nil, fmt.Errorf("ethclient CallContract: %w", err)
	}
	return out, nil
}

func (ec *Client) StringifyDenomination(d sdkTypes.Denomination) string {
	panic("not implemented")
}

func NewClient(ctx context.Context, url string) (*Client, error) {
	client, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("ethclient DialContext %s: %w", url, err)
	}
	return &Client{
		client: client,
	}, nil
}
