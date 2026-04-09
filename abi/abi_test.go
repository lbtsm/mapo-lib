package abi

import (
	"testing"
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
	a, _ := New(testABI)
	data, err := a.PackInput("totalSupply")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 4 {
		t.Fatalf("expected 4 bytes selector, got %d", len(data))
	}
}

func TestGetMethodID(t *testing.T) {
	a, _ := New(testABI)
	id, err := a.GetMethodID("transfer")
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(id))
	}
}

func TestGetMethodIDNotFound(t *testing.T) {
	a, _ := New(testABI)
	_, err := a.GetMethodID("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetEventID(t *testing.T) {
	a, _ := New(testABI)
	topic, err := a.GetEventID("Transfer")
	if err != nil {
		t.Fatal(err)
	}
	if topic == ([32]byte{}) {
		t.Fatal("expected non-zero topic")
	}
}

func TestMethodsAndEvents(t *testing.T) {
	a, _ := New(testABI)
	methods := a.Methods()
	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(methods))
	}
	events := a.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestHasMethodAndEvent(t *testing.T) {
	a, _ := New(testABI)
	if !a.HasMethod("transfer") {
		t.Fatal("expected HasMethod(transfer) = true")
	}
	if a.HasMethod("nonexistent") {
		t.Fatal("expected HasMethod(nonexistent) = false")
	}
	if !a.HasEvent("Transfer") {
		t.Fatal("expected HasEvent(Transfer) = true")
	}
	if a.HasEvent("nonexistent") {
		t.Fatal("expected HasEvent(nonexistent) = false")
	}
}

func TestUnpackInputData(t *testing.T) {
	a, _ := New(testABI)
	// pack a totalSupply call, then unpack it
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
	a, _ := New(testABI)
	id, _ := a.GetMethodID("transfer")
	m, err := a.MethodByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "transfer" {
		t.Fatalf("expected transfer, got %s", m.Name)
	}
}
