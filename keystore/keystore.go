package keystore

import (
	"fmt"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/keystore"
)

const (
	EnvPassword = "KEYSTORE_PASSWORD"
)

var (
	pswCacheMu sync.RWMutex
	pswCache   = make(map[string][]byte)
)

// KeypairFromEthWithPassword decrypts an eth keystore JSON file at path using
// the provided password. It performs no interactive IO. On success the password
// is stored in the in-memory cache keyed by path.
func KeypairFromEthWithPassword(path string, password []byte) (*keystore.Key, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key file not found: %s", path)
		}
		return nil, fmt.Errorf("stat keyFile failed, err:%s", err)
	}

	// Fast path: if we already have a cached password for this path, prefer it
	// over the caller-supplied one. This makes repeated calls cheap and lets
	// KeypairFromEth share the same code path.
	pswCacheMu.RLock()
	if cached, ok := pswCache[path]; ok && len(cached) > 0 {
		password = cached
	}
	pswCacheMu.RUnlock()

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read keyFile failed, err:%s", err)
	}
	ret, err := keystore.DecryptKey(file, string(password))
	if err != nil {
		return nil, fmt.Errorf("DecryptKey failed, err:%s", err)
	}

	pswCacheMu.Lock()
	pswCache[path] = password
	pswCacheMu.Unlock()

	return ret, nil
}

// KeypairFromEth decrypts an eth keystore JSON file at path. It resolves the
// password in this order:
//  1. in-memory cache (previous successful decrypt of the same path)
//  2. KEYSTORE_PASSWORD environment variable
//  3. interactive terminal prompt
//
// On success the password is cached so subsequent calls do not re-prompt.
func KeypairFromEth(path string) (*keystore.Key, error) {
	// 1) cache
	pswCacheMu.RLock()
	pswd := pswCache[path]
	pswCacheMu.RUnlock()

	// 2) env
	if len(pswd) == 0 {
		if env := os.Getenv(EnvPassword); env != "" {
			pswd = []byte(env)
		}
	}

	// 3) interactive prompt
	if len(pswd) == 0 {
		pswd = GetPassword(fmt.Sprintf("Enter password for key %s:", path))
	}

	return KeypairFromEthWithPassword(path, pswd)
}
