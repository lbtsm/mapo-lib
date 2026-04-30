// Package abi wraps go-ethereum's accounts/abi with ergonomic helpers for
// packing inputs, unpacking outputs, decoding transaction calldata, and
// unpacking event logs.
//
// Errors returned for missing methods or events use sentinel values
// (ErrMethodNotFound, ErrEventNotFound) so callers can use errors.Is to
// branch programmatically.
package abi
