package keystore

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
)

// makeTestKeyfile creates an eth keystore JSON file in a temp dir using the
// go-ethereum keystore with LightScrypt params (fast for tests). It returns the
// absolute path to the generated key file and the address for verification.
func makeTestKeyfile(t *testing.T, password string) (path string, address string) {
	t.Helper()
	dir := t.TempDir()
	ks := keystore.NewKeyStore(dir, keystore.LightScryptN, keystore.LightScryptP)
	acc, err := ks.NewAccount(password)
	if err != nil {
		t.Fatalf("NewAccount failed: %v", err)
	}
	return acc.URL.Path, acc.Address.Hex()
}

// resetCache clears the global password cache between tests so they don't
// contaminate each other.
func resetCache() {
	pswCacheMu.Lock()
	defer pswCacheMu.Unlock()
	pswCache = make(map[string][]byte)
}

func TestKeypairFromEthWithPassword_OK(t *testing.T) {
	resetCache()
	const pwd = "correct-horse-battery-staple"
	path, addr := makeTestKeyfile(t, pwd)

	key, err := KeypairFromEthWithPassword(path, []byte(pwd))
	if err != nil {
		t.Fatalf("KeypairFromEthWithPassword failed: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
	if got := key.Address.Hex(); got != addr {
		t.Fatalf("address mismatch: got %s want %s", got, addr)
	}
}

func TestKeypairFromEthWithPassword_WrongPassword(t *testing.T) {
	resetCache()
	path, _ := makeTestKeyfile(t, "right-password")

	key, err := KeypairFromEthWithPassword(path, []byte("wrong-password"))
	if err == nil {
		t.Fatalf("expected error for wrong password, got key=%v", key)
	}

	// Cache must NOT be populated on failure.
	pswCacheMu.RLock()
	_, cached := pswCache[path]
	pswCacheMu.RUnlock()
	if cached {
		t.Fatal("password cache should not be populated when decryption fails")
	}
}

func TestKeypairFromEthWithPassword_FileNotFound(t *testing.T) {
	resetCache()
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")

	if _, err := KeypairFromEthWithPassword(missing, []byte("any")); err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestKeypairFromEthWithPassword_CacheHit(t *testing.T) {
	resetCache()
	const pwd = "cache-test-pwd"
	path, _ := makeTestKeyfile(t, pwd)

	first, err := KeypairFromEthWithPassword(path, []byte(pwd))
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Verify cache is populated.
	pswCacheMu.RLock()
	cached, ok := pswCache[path]
	pswCacheMu.RUnlock()
	if !ok || string(cached) != pwd {
		t.Fatalf("expected cache to contain %q, got ok=%v val=%q", pwd, ok, string(cached))
	}

	second, err := KeypairFromEthWithPassword(path, []byte(pwd))
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if first.Address != second.Address {
		t.Fatalf("address differs across calls: %s vs %s", first.Address.Hex(), second.Address.Hex())
	}
}

func TestKeypairFromEth_EnvVarFallback(t *testing.T) {
	resetCache()
	const pwd = "env-var-pwd"
	path, addr := makeTestKeyfile(t, pwd)

	t.Setenv(EnvPassword, pwd)

	key, err := KeypairFromEth(path)
	if err != nil {
		t.Fatalf("KeypairFromEth failed: %v", err)
	}
	if key.Address.Hex() != addr {
		t.Fatalf("address mismatch: got %s want %s", key.Address.Hex(), addr)
	}

	// Confirm it landed in the cache too.
	pswCacheMu.RLock()
	cached, ok := pswCache[path]
	pswCacheMu.RUnlock()
	if !ok || string(cached) != pwd {
		t.Fatalf("expected cache populated from env, got ok=%v val=%q", ok, string(cached))
	}
}

func TestKeypairFromEth_CacheBeatsEnv(t *testing.T) {
	resetCache()
	const pwd = "real-pwd"
	path, _ := makeTestKeyfile(t, pwd)

	// Prime the cache with the correct password.
	pswCacheMu.Lock()
	pswCache[path] = []byte(pwd)
	pswCacheMu.Unlock()

	// Set env to a wrong password — cache should win, no error.
	t.Setenv(EnvPassword, "this-would-fail-if-used")

	if _, err := KeypairFromEth(path); err != nil {
		t.Fatalf("expected cache to short-circuit env fallback, got err: %v", err)
	}
}

func TestPswCacheConcurrent(t *testing.T) {
	resetCache()
	const pwd = "concurrent-pwd"
	path, _ := makeTestKeyfile(t, pwd)

	const N = 50
	var wg sync.WaitGroup
	errs := make(chan error, N)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			if _, err := KeypairFromEthWithPassword(path, []byte(pwd)); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent call failed: %v", err)
	}
}

// Sanity: ensure tests don't leak the env var to other tests when they don't
// set it themselves.
func TestEnvUnsetByDefault(t *testing.T) {
	if v, ok := os.LookupEnv(EnvPassword); ok {
		t.Logf("note: %s is set in test environment to %q", EnvPassword, v)
	}
}
