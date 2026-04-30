package contract

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/params"
	mapoabi "github.com/lbtsm/mapo-lib/abi"
)

// valueABI is the JSON ABI for the test contract: a single view method
// `value()` that returns a uint256 (always 42 in our deployed bytecode).
const valueABI = `[
    {
        "inputs": [],
        "name": "value",
        "outputs": [{"internalType":"uint256","name":"","type":"uint256"}],
        "stateMutability": "view",
        "type": "function"
    }
]`

// valueDeployBytecode is hand-rolled EVM bytecode (no solc needed) that
// deploys a runtime which unconditionally returns the 32-byte value 42.
//
// Init code:
//
//	PUSH1 0x0a    // runtime length = 10
//	PUSH1 0x0c    // runtime offset in init code = 12
//	PUSH1 0x00
//	CODECOPY
//	PUSH1 0x0a
//	PUSH1 0x00
//	RETURN
//
// Runtime: PUSH1 0x2a; PUSH1 0x00; MSTORE; PUSH1 0x20; PUSH1 0x00; RETURN
const valueDeployBytecode = "600a600c600039600a6000f3602a60005260206000f3"

// callerForTest is a thin shim allowing tests to use the simulated.Client
// (which is an interface, not *ethclient.Client) as the contract caller.
// It is satisfied by both *ethclient.Client and simulated.Client.
type callerForTest interface {
	ethereum.ContractCaller
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

// simEnv bundles a simulated backend with a deployed value contract.
type simEnv struct {
	backend  *simulated.Backend
	client   callerForTest
	contract common.Address
	abi      *mapoabi.Abi
	owner    common.Address
}

func setupSim(t *testing.T) *simEnv {
	t.Helper()

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	owner := crypto.PubkeyToAddress(key.PublicKey)

	backend := simulated.NewBackend(types.GenesisAlloc{
		owner: {Balance: new(big.Int).Mul(big.NewInt(1_000), big.NewInt(1e18))},
	})
	t.Cleanup(func() { _ = backend.Close() })

	client := backend.Client()

	// Build a contract-creation transaction with our hand-rolled bytecode.
	ctx := context.Background()
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatalf("HeaderByNumber: %v", err)
	}
	gasPrice := new(big.Int).Add(head.BaseFee, big.NewInt(params.GWei))
	signer := types.LatestSigner(params.AllDevChainProtocolChanges)

	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce:    0,
		Gas:      300_000,
		GasPrice: gasPrice,
		Data:     common.FromHex(valueDeployBytecode),
	})
	if err := client.SendTransaction(ctx, tx); err != nil {
		t.Fatalf("SendTransaction: %v", err)
	}
	backend.Commit()

	receipt, err := client.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		t.Fatalf("TransactionReceipt: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("deploy failed, status=%d", receipt.Status)
	}
	if receipt.ContractAddress == (common.Address{}) {
		t.Fatal("no contract address in receipt")
	}

	a, err := mapoabi.New(valueABI)
	if err != nil {
		t.Fatalf("abi.New: %v", err)
	}

	return &simEnv{
		backend:  backend,
		client:   client,
		contract: receipt.ContractAddress,
		abi:      a,
		owner:    owner,
	}
}

// newCallForTest constructs a *Call backed by the simulated client. The
// public New requires *ethclient.Client, but simulated.Client is an interface
// — so internally Call must accept ethereum.ContractCaller.
func newCallForTest(env *simEnv, opts ...Option) *Call {
	return newWithCaller(env.client, []common.Address{env.contract}, env.abi, opts...)
}

func TestCallContract_OK(t *testing.T) {
	env := setupSim(t)
	c := newCallForTest(env)

	var got *big.Int
	if err := c.CallContract(context.Background(), env.contract, "value", &got); err != nil {
		t.Fatalf("CallContract: %v", err)
	}
	if got == nil || got.Int64() != 42 {
		t.Fatalf("got %v, want 42", got)
	}
}

func TestCall_BackwardCompat(t *testing.T) {
	env := setupSim(t)
	c := newCallForTest(env)

	var got *big.Int
	if err := c.Call("value", &got, 0); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got == nil || got.Int64() != 42 {
		t.Fatalf("got %v, want 42", got)
	}
}

func TestCall_BadIndex(t *testing.T) {
	env := setupSim(t)
	c := newCallForTest(env)

	var got *big.Int

	// Index beyond range must return an error, never panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Call panicked on bad index: %v", r)
		}
	}()

	err := c.Call("value", &got, 99)
	if err == nil {
		t.Fatal("expected error for out-of-range index, got nil")
	}

	// Negative index too.
	if err := c.Call("value", &got, -1); err == nil {
		t.Fatal("expected error for negative index, got nil")
	}
}

func TestCallAt_AtBlock(t *testing.T) {
	env := setupSim(t)
	c := newCallForTest(env)

	// Mine a few extra blocks so we have a real history.
	env.backend.Commit()
	env.backend.Commit()

	ctx := context.Background()
	head, err := env.client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatalf("HeaderByNumber: %v", err)
	}

	var got *big.Int
	if err := c.CallAt(ctx, "value", &got, 0, head.Number); err != nil {
		t.Fatalf("CallAt at head: %v", err)
	}
	if got == nil || got.Int64() != 42 {
		t.Fatalf("got %v, want 42", got)
	}

	// Querying genesis (block 0) — the contract was deployed *after* genesis,
	// so the call must NOT return 42. Either an error or a non-42 result is
	// acceptable; the point is CallAt actually targets the requested block.
	var atGenesis *big.Int
	err = c.CallAt(ctx, "value", &atGenesis, 0, big.NewInt(0))
	if err == nil && atGenesis != nil && atGenesis.Int64() == 42 {
		t.Fatal("expected genesis call to fail or return non-42 (contract not yet deployed)")
	}
}

func TestCall_MethodNotFound(t *testing.T) {
	env := setupSim(t)
	c := newCallForTest(env)

	var got *big.Int
	err := c.Call("doesNotExist", &got, 0)
	if err == nil {
		t.Fatal("expected error for unknown method, got nil")
	}
}

func TestCall_ContextCanceled(t *testing.T) {
	env := setupSim(t)
	c := newCallForTest(env)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var got *big.Int
	err := c.CallAt(ctx, "value", &got, 0, nil)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}

func TestWithFrom(t *testing.T) {
	env := setupSim(t)

	from := common.HexToAddress("0xCAfEBabE00000000000000000000000000000001")
	c := newCallForTest(env, WithFrom(from))

	var got *big.Int
	if err := c.CallContract(context.Background(), env.contract, "value", &got); err != nil {
		t.Fatalf("CallContract w/ WithFrom: %v", err)
	}
	if got == nil || got.Int64() != 42 {
		t.Fatalf("got %v, want 42", got)
	}

	// Also verify the option is captured on the struct.
	if c.from != from {
		t.Fatalf("from = %s, want %s", c.from.Hex(), from.Hex())
	}
}

func TestWithFrom_DefaultZero(t *testing.T) {
	env := setupSim(t)
	c := newCallForTest(env)

	if c.from != (common.Address{}) {
		t.Fatalf("default from = %s, want zero address", c.from.Hex())
	}
}
