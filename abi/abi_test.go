package abi

import (
	"errors"
	"math/big"
	"testing"

	gethabi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const testABI = `[
	{
		"inputs": [{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],
		"name": "transfer",
		"outputs": [{"internalType":"bool","name":"","type":"bool"}],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "totalSupply",
		"outputs": [{"internalType":"uint256","name":"","type":"uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed":true,"internalType":"address","name":"from","type":"address"},
			{"indexed":true,"internalType":"address","name":"to","type":"address"},
			{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}
		],
		"name": "Transfer",
		"type": "event"
	}
]`

// mustNew is a small test helper; t.Helper() ensures failures point at the call site.
func mustNew(t *testing.T) *Abi {
	t.Helper()
	a, err := New(testABI)
	if err != nil {
		t.Fatalf("New(testABI) failed: %v", err)
	}
	return a
}

func TestNew(t *testing.T) {
	a, err := New(testABI)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected non-nil Abi")
	}
}

func TestNewInvalid(t *testing.T) {
	_, err := New("not json")
	if err == nil {
		t.Fatal("expected error for invalid ABI")
	}
}

func TestPackInput(t *testing.T) {
	a := mustNew(t)
	data, err := a.PackInput("totalSupply")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 4 {
		t.Fatalf("expected 4 bytes selector, got %d", len(data))
	}
}

// TestGetMethodID covers both the success and not-found paths in one table.
func TestGetMethodID(t *testing.T) {
	a := mustNew(t)
	tests := []struct {
		name     string
		method   string
		wantErr  bool
		wantSize int
	}{
		{name: "transfer found", method: "transfer", wantSize: 4},
		{name: "totalSupply found", method: "totalSupply", wantSize: 4},
		{name: "nonexistent returns error", method: "nonexistent", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := a.GetMethodID(tt.method)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrMethodNotFound) {
					t.Fatalf("expected ErrMethodNotFound, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(id) != tt.wantSize {
				t.Fatalf("expected %d bytes, got %d", tt.wantSize, len(id))
			}
		})
	}
}

func TestGetEventID(t *testing.T) {
	a := mustNew(t)
	topic, err := a.GetEventID("Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if topic == (common.Hash{}) {
		t.Fatal("expected non-zero topic")
	}
}

func TestGetEventIDNotFound(t *testing.T) {
	a := mustNew(t)
	_, err := a.GetEventID("Nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("expected ErrEventNotFound, got %v", err)
	}
}

func TestMethodsAndEvents(t *testing.T) {
	a := mustNew(t)
	methods := a.Methods()
	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(methods))
	}
	events := a.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

// TestHasMethodAndEvent table-driven combo for both Has* checks.
func TestHasMethodAndEvent(t *testing.T) {
	a := mustNew(t)
	tests := []struct {
		name string
		kind string // "method" or "event"
		key  string
		want bool
	}{
		{name: "transfer is a method", kind: "method", key: "transfer", want: true},
		{name: "totalSupply is a method", kind: "method", key: "totalSupply", want: true},
		{name: "missing method", kind: "method", key: "nonexistent", want: false},
		{name: "Transfer is an event", kind: "event", key: "Transfer", want: true},
		{name: "missing event", kind: "event", key: "nonexistent", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			switch tt.kind {
			case "method":
				got = a.HasMethod(tt.key)
			case "event":
				got = a.HasEvent(tt.key)
			}
			if got != tt.want {
				t.Fatalf("Has%s(%q) = %v, want %v", tt.kind, tt.key, got, tt.want)
			}
		})
	}
}

func TestUnpackInputData(t *testing.T) {
	a := mustNew(t)
	data, _ := a.PackInput("totalSupply")
	name, values, err := a.UnpackInputData(data)
	if err != nil {
		t.Fatal(err)
	}
	if name != "totalSupply" {
		t.Fatalf("expected totalSupply, got %s", name)
	}
	if len(values) != 0 {
		t.Fatalf("expected 0 values, got %d", len(values))
	}
}

func TestMethodByID(t *testing.T) {
	a := mustNew(t)
	id, _ := a.GetMethodID("transfer")
	m, err := a.MethodByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "transfer" {
		t.Fatalf("expected transfer, got %s", m.Name)
	}
}

// transferEvent matches the shape of the Transfer event in testABI.
type transferEvent struct {
	From  common.Address
	To    common.Address
	Value *big.Int
}

// buildTransferLog crafts a real Transfer log: indexed args go in topics, the
// non-indexed value goes in data. We reach into the embedded go-ethereum ABI to
// pack the data, mirroring what an EVM node would emit.
func buildTransferLog(t *testing.T, a *Abi, from, to common.Address, value *big.Int) types.Log {
	t.Helper()
	ev, ok := a.ABI().Events["Transfer"]
	if !ok {
		t.Fatal("Transfer event missing from ABI")
	}
	// Only the non-indexed inputs appear in Data.
	var nonIndexed gethabi.Arguments
	for _, arg := range ev.Inputs {
		if !arg.Indexed {
			nonIndexed = append(nonIndexed, arg)
		}
	}
	data, err := nonIndexed.Pack(value)
	if err != nil {
		t.Fatalf("pack non-indexed: %v", err)
	}
	return types.Log{
		Topics: []common.Hash{
			ev.ID,
			common.BytesToHash(from.Bytes()),
			common.BytesToHash(to.Bytes()),
		},
		Data: data,
	}
}

func TestUnpackLog(t *testing.T) {
	a := mustNew(t)
	from := common.HexToAddress("0x1111111111111111111111111111111111111111")
	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	value := big.NewInt(987654321)

	log := buildTransferLog(t, a, from, to, value)

	var got transferEvent
	if err := a.UnpackLog(&got, "Transfer", log); err != nil {
		t.Fatalf("UnpackLog: %v", err)
	}
	if got.From != from {
		t.Errorf("From = %s, want %s", got.From.Hex(), from.Hex())
	}
	if got.To != to {
		t.Errorf("To = %s, want %s", got.To.Hex(), to.Hex())
	}
	if got.Value == nil || got.Value.Cmp(value) != 0 {
		t.Errorf("Value = %v, want %v", got.Value, value)
	}
}

func TestUnpackLog_NoTopics(t *testing.T) {
	a := mustNew(t)

	// Build a Data-only log with no topics. Transfer has indexed args, so this
	// must be rejected as malformed without panicking.
	ev := a.ABI().Events["Transfer"]
	var nonIndexed gethabi.Arguments
	for _, arg := range ev.Inputs {
		if !arg.Indexed {
			nonIndexed = append(nonIndexed, arg)
		}
	}
	data, err := nonIndexed.Pack(big.NewInt(1))
	if err != nil {
		t.Fatalf("pack: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UnpackLog panicked on empty topics: %v", r)
		}
	}()

	var out transferEvent
	err = a.UnpackLog(&out, "Transfer", types.Log{Topics: nil, Data: data})
	if err == nil {
		t.Fatal("expected error for log with no topics, got nil")
	}
}

func TestUnpackOutput(t *testing.T) {
	a := mustNew(t)
	m := a.ABI().Methods["totalSupply"]
	want := big.NewInt(12345)
	output, err := m.Outputs.Pack(want)
	if err != nil {
		t.Fatalf("pack output: %v", err)
	}

	var got *big.Int
	if err := a.UnpackOutput("totalSupply", &got, output); err != nil {
		t.Fatalf("UnpackOutput: %v", err)
	}
	if got == nil || got.Cmp(want) != 0 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnpackOutputValues(t *testing.T) {
	a := mustNew(t)
	m := a.ABI().Methods["totalSupply"]
	want := big.NewInt(12345)
	output, err := m.Outputs.Pack(want)
	if err != nil {
		t.Fatalf("pack output: %v", err)
	}

	values, err := a.UnpackOutputValues("totalSupply", output)
	if err != nil {
		t.Fatalf("UnpackOutputValues: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}
	got, ok := values[0].(*big.Int)
	if !ok {
		t.Fatalf("expected *big.Int, got %T", values[0])
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnpackEventValues(t *testing.T) {
	a := mustNew(t)
	ev := a.ABI().Events["Transfer"]
	var nonIndexed gethabi.Arguments
	for _, arg := range ev.Inputs {
		if !arg.Indexed {
			nonIndexed = append(nonIndexed, arg)
		}
	}
	want := big.NewInt(42)
	data, err := nonIndexed.Pack(want)
	if err != nil {
		t.Fatalf("pack event data: %v", err)
	}

	values, err := a.UnpackEventValues("Transfer", data)
	if err != nil {
		t.Fatalf("UnpackEventValues: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}
	got, ok := values[0].(*big.Int)
	if !ok {
		t.Fatalf("expected *big.Int, got %T", values[0])
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnpackInput(t *testing.T) {
	a := mustNew(t)
	to := common.HexToAddress("0x3333333333333333333333333333333333333333")
	amount := big.NewInt(7777)

	full, err := a.PackInput("transfer", to, amount)
	if err != nil {
		t.Fatalf("PackInput: %v", err)
	}
	if len(full) < 4 {
		t.Fatalf("packed input too short: %d", len(full))
	}

	values, err := a.UnpackInput("transfer", full[4:])
	if err != nil {
		t.Fatalf("UnpackInput: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
	gotTo, ok := values[0].(common.Address)
	if !ok {
		t.Fatalf("values[0] type = %T, want common.Address", values[0])
	}
	if gotTo != to {
		t.Fatalf("to = %s, want %s", gotTo.Hex(), to.Hex())
	}
	gotAmt, ok := values[1].(*big.Int)
	if !ok {
		t.Fatalf("values[1] type = %T, want *big.Int", values[1])
	}
	if gotAmt.Cmp(amount) != 0 {
		t.Fatalf("amount = %v, want %v", gotAmt, amount)
	}
}

func TestEventByID(t *testing.T) {
	a := mustNew(t)
	topic, err := a.GetEventID("Transfer")
	if err != nil {
		t.Fatal(err)
	}
	ev, err := a.EventByID(topic)
	if err != nil {
		t.Fatalf("EventByID: %v", err)
	}
	if ev.Name != "Transfer" {
		t.Fatalf("event name = %s, want Transfer", ev.Name)
	}
}

func TestEventByID_NotFound(t *testing.T) {
	a := mustNew(t)
	// Hash of a string that is definitely not in the ABI.
	bogus := crypto.Keccak256Hash([]byte("DefinitelyNotAnEvent(uint256)"))
	if _, err := a.EventByID(bogus); err == nil {
		t.Fatal("expected error for unknown topic")
	}
}

func TestErrMethodNotFound_IsErr(t *testing.T) {
	a := mustNew(t)
	_, err := a.GetMethodID("nope")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrMethodNotFound) {
		t.Fatalf("errors.Is(err, ErrMethodNotFound) = false; err = %v", err)
	}
}

func TestErrEventNotFound_IsErr(t *testing.T) {
	a := mustNew(t)
	_, err := a.UnpackEventValues("nope", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("errors.Is(err, ErrEventNotFound) = false; err = %v", err)
	}
}

// FuzzNew ensures New never panics for arbitrary input strings.
func FuzzNew(f *testing.F) {
	f.Add(testABI)
	f.Add("")
	f.Add("not json")
	f.Add("[]")
	f.Add("[{}]")
	f.Add(`[{"type":"function"}]`)
	f.Add(`{"foo":"bar"}`)
	f.Fuzz(func(t *testing.T, s string) {
		// Bound input size — go-ethereum's parser walks the whole string.
		if len(s) > 64*1024 {
			t.Skip()
		}
		// New must return either (Abi, nil) or (nil, err); never panic.
		a, err := New(s)
		if err == nil && a == nil {
			t.Fatal("New returned (nil, nil)")
		}
	})
}

func BenchmarkPackInput(b *testing.B) {
	a, err := New(testABI)
	if err != nil {
		b.Fatal(err)
	}
	to := common.HexToAddress("0x4444444444444444444444444444444444444444")
	amount := big.NewInt(1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.PackInput("transfer", to, amount); err != nil {
			b.Fatal(err)
		}
	}
}
