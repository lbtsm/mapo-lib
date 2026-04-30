// Package contract is a thin read-only contract caller built on go-ethereum.
//
// It wraps an ethclient (or any ethereum.ContractCaller) plus a parsed ABI
// and exposes ergonomic methods for invoking view/pure functions and
// decoding their outputs. The caller does not sign or submit transactions.
package contract

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	mapoabi "github.com/lbtsm/mapo-lib/abi"
)

// ErrIndexOutOfRange is returned by Call/CallAt when the supplied address
// index is outside the bounds of the configured address slice. It replaces
// the panic that the previous implementation produced.
var ErrIndexOutOfRange = errors.New("contract: address index out of range")

// Option configures a Call at construction time.
type Option func(*Call)

// WithFrom sets the From field on outgoing eth_call CallMsgs. The EVM
// honors From for calls that branch on msg.sender (for example, role
// checks). Default: the zero address.
func WithFrom(from common.Address) Option {
	return func(c *Call) {
		c.from = from
	}
}

// Call is a read-only contract caller bound to one ABI and a slice of
// contract addresses. It is safe for concurrent use as long as the
// underlying client is.
type Call struct {
	abi    *mapoabi.Abi
	addrs  []common.Address
	client ethereum.ContractCaller
	from   common.Address
}

// New creates a contract caller with the given client, addresses, ABI, and
// options. The address slice is retained by index for use with the
// backward-compatible Call/CallAt methods; new code should prefer
// CallContract, which takes the address explicitly.
func New(client *ethclient.Client, addrs []common.Address, abi *mapoabi.Abi, opts ...Option) *Call {
	return newWithCaller(client, addrs, abi, opts...)
}

// newWithCaller is the internal constructor that accepts any
// ethereum.ContractCaller, including the simulated client used in tests.
func newWithCaller(client ethereum.ContractCaller, addrs []common.Address, abi *mapoabi.Abi, opts ...Option) *Call {
	c := &Call{
		client: client,
		addrs:  addrs,
		abi:    abi,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CallContract executes a read-only call against addr. It is the preferred
// API: callers don't need to know about the address slice or its ordering.
// If blockNumber-style targeting is needed, see CallAt; CallContract always
// uses the latest block.
func (c *Call) CallContract(ctx context.Context, addr common.Address, method string, ret any, params ...any) error {
	return c.callAt(ctx, addr, method, ret, nil, params...)
}

// Call executes a read-only contract call on the address at the given
// index. Kept for backward compatibility; prefer CallContract.
//
// Returns ErrIndexOutOfRange if idx is out of range (previously this would
// panic with an index-out-of-range runtime error).
func (c *Call) Call(method string, ret any, idx int, params ...any) error {
	return c.CallAt(context.Background(), method, ret, idx, nil, params...)
}

// CallAt executes a read-only contract call at a specific block number. If
// blockNumber is nil, the latest block is used. Kept for backward
// compatibility; prefer CallContract for new code.
//
// Returns ErrIndexOutOfRange if idx is out of range.
func (c *Call) CallAt(ctx context.Context, method string, ret any, idx int, blockNumber *big.Int, params ...any) error {
	if idx < 0 || idx >= len(c.addrs) {
		return fmt.Errorf("%w: idx=%d len=%d", ErrIndexOutOfRange, idx, len(c.addrs))
	}
	return c.callAt(ctx, c.addrs[idx], method, ret, blockNumber, params...)
}

// callAt is the single underlying implementation used by both the
// index-based legacy API and the address-based CallContract.
func (c *Call) callAt(ctx context.Context, addr common.Address, method string, ret any, blockNumber *big.Int, params ...any) error {
	input, err := c.abi.PackInput(method, params...)
	if err != nil {
		return err
	}

	output, err := c.client.CallContract(ctx,
		ethereum.CallMsg{
			From: c.from,
			To:   &addr,
			Data: input,
		},
		blockNumber,
	)
	if err != nil {
		return err
	}

	return c.abi.UnpackOutput(method, ret, output)
}

// Address returns the contract address at the given index. For
// out-of-range indices it returns the zero address rather than panicking.
func (c *Call) Address(idx int) common.Address {
	if idx < 0 || idx >= len(c.addrs) {
		return common.Address{}
	}
	return c.addrs[idx]
}

// Addresses returns all contract addresses.
func (c *Call) Addresses() []common.Address {
	return c.addrs
}
