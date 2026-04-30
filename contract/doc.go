// Package contract is a read-only contract caller built on go-ethereum's
// ethclient and the abi package in this module.
//
// The preferred entry point is CallContract, which takes the contract address
// explicitly. The legacy index-based methods Call and CallAt are kept for
// backwards compatibility and now return ErrIndexOutOfRange instead of
// panicking on a bad index.
package contract
