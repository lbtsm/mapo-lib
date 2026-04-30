package abi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Sentinel errors. Wrapped with %w so callers can use errors.Is.
var (
	ErrMethodNotFound = errors.New("method not found")
	ErrEventNotFound  = errors.New("event not found")
	// ErrMissingTopics is returned by UnpackLog when a log lacks the topics
	// required to decode the event's indexed fields.
	ErrMissingTopics = errors.New("log is missing required topics")
)

type Abi struct {
	abi abi.ABI
}

// New creates an Abi instance from a JSON ABI string.
func New(abiStr string) (*Abi, error) {
	a, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return nil, err
	}
	return &Abi{abi: a}, nil
}

// ABI returns the underlying go-ethereum ABI.
func (a *Abi) ABI() abi.ABI {
	return a.abi
}

// PackInput packs a method call with the given parameters.
func (a *Abi) PackInput(method string, params ...any) ([]byte, error) {
	return a.abi.Pack(method, params...)
}

// UnpackInput unpacks the input data of a method call (without the 4-byte selector).
func (a *Abi) UnpackInput(method string, data []byte) ([]any, error) {
	m, ok := a.abi.Methods[method]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMethodNotFound, method)
	}
	return m.Inputs.Unpack(data)
}

// UnpackInputData unpacks transaction input data (with the 4-byte selector).
func (a *Abi) UnpackInputData(data []byte) (string, []any, error) {
	if len(data) < 4 {
		return "", nil, fmt.Errorf("data too short: %d bytes", len(data))
	}
	m, err := a.abi.MethodById(data[:4])
	if err != nil {
		return "", nil, err
	}
	values, err := m.Inputs.Unpack(data[4:])
	if err != nil {
		return "", nil, err
	}
	return m.Name, values, nil
}

// UnpackOutput unpacks the output of a method call into the given struct or variable.
func (a *Abi) UnpackOutput(method string, ret any, output []byte) error {
	m, ok := a.abi.Methods[method]
	if !ok {
		return fmt.Errorf("%w: %s", ErrMethodNotFound, method)
	}
	unpack, err := m.Outputs.Unpack(output)
	if err != nil {
		return fmt.Errorf("unpack output: %w", err)
	}
	if err = m.Outputs.Copy(ret, unpack); err != nil {
		return fmt.Errorf("copy output: %w", err)
	}
	return nil
}

// UnpackOutputValues unpacks the output of a method call into a list of values.
func (a *Abi) UnpackOutputValues(method string, output []byte) ([]any, error) {
	m, ok := a.abi.Methods[method]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMethodNotFound, method)
	}
	return m.Outputs.Unpack(output)
}

// UnpackEventValues unpacks event data (non-indexed fields) into a list of values.
func (a *Abi) UnpackEventValues(event string, data []byte) ([]any, error) {
	e, ok := a.abi.Events[event]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrEventNotFound, event)
	}
	return e.Inputs.UnpackValues(data)
}

// UnpackLog unpacks a log entry into the given struct.
//
// The log's first topic (if present) is the event signature; the remaining
// topics carry indexed arguments. Empty/missing topics on an event with
// indexed args yields ErrMissingTopics rather than a panic.
func (a *Abi) UnpackLog(out any, event string, log types.Log) error {
	e, ok := a.abi.Events[event]
	if !ok {
		return fmt.Errorf("%w: %s", ErrEventNotFound, event)
	}
	// unpack non-indexed fields from data
	if len(log.Data) > 0 {
		if err := a.abi.UnpackIntoInterface(out, event, log.Data); err != nil {
			return fmt.Errorf("unpack log data: %w", err)
		}
	}
	// gather indexed arguments
	var indexed abi.Arguments
	for _, arg := range e.Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	if len(indexed) == 0 {
		return nil
	}
	// We need topics[0] (signature) plus one topic per indexed arg.
	if len(log.Topics) < 1+len(indexed) {
		return fmt.Errorf("%w: have %d, need %d for event %s",
			ErrMissingTopics, len(log.Topics), 1+len(indexed), event)
	}
	if err := abi.ParseTopics(out, indexed, log.Topics[1:]); err != nil {
		return fmt.Errorf("parse indexed topics: %w", err)
	}
	return nil
}

// GetMethodID returns the 4-byte selector for a method.
func (a *Abi) GetMethodID(method string) ([]byte, error) {
	m, ok := a.abi.Methods[method]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMethodNotFound, method)
	}
	return m.ID, nil
}

// GetEventID returns the topic hash (event signature) for an event.
func (a *Abi) GetEventID(event string) (common.Hash, error) {
	e, ok := a.abi.Events[event]
	if !ok {
		return common.Hash{}, fmt.Errorf("%w: %s", ErrEventNotFound, event)
	}
	return e.ID, nil
}

// MethodByID looks up a method by its 4-byte selector.
func (a *Abi) MethodByID(id []byte) (*abi.Method, error) {
	return a.abi.MethodById(id)
}

// EventByID looks up an event by its topic hash.
func (a *Abi) EventByID(topic common.Hash) (*abi.Event, error) {
	return a.abi.EventByID(topic)
}

// Methods returns all method names in this ABI.
func (a *Abi) Methods() []string {
	names := make([]string, 0, len(a.abi.Methods))
	for name := range a.abi.Methods {
		names = append(names, name)
	}
	return names
}

// Events returns all event names in this ABI.
func (a *Abi) Events() []string {
	names := make([]string, 0, len(a.abi.Events))
	for name := range a.abi.Events {
		names = append(names, name)
	}
	return names
}

// HasMethod checks if the ABI contains the given method.
func (a *Abi) HasMethod(method string) bool {
	_, ok := a.abi.Methods[method]
	return ok
}

// HasEvent checks if the ABI contains the given event.
func (a *Abi) HasEvent(event string) bool {
	_, ok := a.abi.Events[event]
	return ok
}
