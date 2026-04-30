// Package keystore decrypts go-ethereum keystore JSON files.
//
// Two entry points are provided:
//
//   - KeypairFromEthWithPassword takes the password as a parameter and is
//     suitable for non-interactive contexts (servers, tests, CI).
//   - KeypairFromEth applies a resolution chain of cache, the
//     KEYSTORE_PASSWORD environment variable, and finally a terminal prompt.
//
// Decrypted passwords are cached per file path and protected by a mutex.
// Failed decryptions do not poison the cache.
package keystore
