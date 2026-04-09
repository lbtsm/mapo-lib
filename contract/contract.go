package contract

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	mapoabi "github.com/mapprotocol/mapo-lib/abi"
)

type Call struct {
	abi    *mapoabi.Abi
	addrs  []common.Address
	client *ethclient.Client
}

// New creates a contract caller with the given client, addresses and ABI.
func New(client *ethclient.Client, addrs []common.Address, abi *mapoabi.Abi) *Call {
	return &Call{
		client: client,
		addrs:  addrs,
		abi:    abi,
	}
}

// Call executes a read-only contract call on the address at the given index.
func (c *Call) Call(method string, ret interface{}, idx int, params ...interface{}) error {
	return c.CallAt(context.Background(), method, ret, idx, nil, params...)
}

// CallAt executes a read-only contract call at a specific block number.
// If blockNumber is nil, the latest block is used.
func (c *Call) CallAt(ctx context.Context, method string, ret interface{}, idx int, blockNumber *big.Int, params ...interface{}) error {
	input, err := c.abi.PackInput(method, params...)
	if err != nil {
		return err
	}

	output, err := c.client.CallContract(ctx,
		ethereum.CallMsg{
			To:   &c.addrs[idx],
			Data: input,
		},
		blockNumber,
	)
	if err != nil {
		return err
	}

	return c.abi.UnpackOutput(method, ret, output)
}

// Address returns the contract address at the given index.
func (c *Call) Address(idx int) common.Address {
	return c.addrs[idx]
}

// Addresses returns all contract addresses.
func (c *Call) Addresses() []common.Address {
	return c.addrs
}
