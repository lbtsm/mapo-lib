package alarm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSend(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	a := New("test-env", srv.URL)
	ctx := context.Background()

	if err := a.Send(ctx, "hello"); err != nil {
		t.Fatalf("first send failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	// duplicate within interval should be suppressed
	if err := a.Send(ctx, "hello"); err != nil {
		t.Fatalf("second send failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected still 1 call, got %d", calls)
	}

	// different message should go through
	if err := a.Send(ctx, "world"); err != nil {
		t.Fatalf("third send failed: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestSendWithShortInterval(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	a := New("env", srv.URL, WithInterval(1*time.Second))
	ctx := context.Background()

	a.Send(ctx, "msg")
	time.Sleep(1100 * time.Millisecond)
	a.Send(ctx, "msg")

	if calls != 2 {
		t.Fatalf("expected 2 calls after interval, got %d", calls)
	}
}

func TestSendEmptyURL(t *testing.T) {
	a := New("env", "")
	err := a.Send(context.Background(), "test")
	if err != nil {
		t.Fatalf("expected nil error for empty URL (should just log), got: %v", err)
	}
}

func TestDefaultAlarm(t *testing.T) {
	t.Cleanup(func() { defaultAlarm.Store(nil) })

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// before init
	err := Send(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error before Init")
	}

	Init("env", srv.URL)
	if err := Send(context.Background(), "hello"); err != nil {
		t.Fatalf("default send failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

// TestSendConcurrent verifies that 100 goroutines sending the same message
// at the same time result in exactly 1 HTTP call, not multiple. This guards
// against a TOCTOU race in the dedup check.
func TestSendConcurrent(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	a := New("env", srv.URL)
	ctx := context.Background()

	const n = 100
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-start
			if err := a.Send(ctx, "boom"); err != nil {
				t.Errorf("send failed: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Fatalf("expected exactly 1 HTTP call, got %d", got)
	}
}

// TestSendDoesNotDedupOnFailure verifies that a failed send is not recorded
// in the dedup map so the next attempt is allowed through.
func TestSendDoesNotDedupOnFailure(t *testing.T) {
	var calls int64
	var fail atomic.Bool
	fail.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("nope"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	a := New("env", srv.URL)
	ctx := context.Background()

	// First call: server fails, should return an error and NOT record dedup.
	if err := a.Send(ctx, "boom"); err == nil {
		t.Fatal("expected first send to fail")
	}
	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Fatalf("expected 1 call after first send, got %d", got)
	}

	// Flip server to success — second call must actually attempt the send,
	// not be deduped from the failed attempt.
	fail.Store(false)
	if err := a.Send(ctx, "boom"); err != nil {
		t.Fatalf("second send failed: %v", err)
	}
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Fatalf("expected 2 calls (failure not recorded as dedup), got %d", got)
	}
}

// TestSeenMapPrunes verifies that the dedup map is pruned of stale entries
// on subsequent Send calls.
func TestSeenMapPrunes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	a := New("env", srv.URL, WithInterval(50*time.Millisecond))
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		if err := a.Send(ctx, string(rune('a'+i))); err != nil {
			t.Fatalf("send %d failed: %v", i, err)
		}
	}
	if got := a.seenSize(); got != 10 {
		t.Fatalf("expected 10 entries before prune, got %d", got)
	}

	// Sleep long enough that all 10 entries are older than 2*interval (=100ms).
	time.Sleep(200 * time.Millisecond)

	// One more send should trigger a prune of the stale entries.
	if err := a.Send(ctx, "fresh"); err != nil {
		t.Fatalf("final send failed: %v", err)
	}
	if got := a.seenSize(); got >= 10 {
		t.Fatalf("expected map to shrink after prune, got %d entries", got)
	}
}

// TestInitConcurrent runs Init and Send concurrently to verify there is no
// data race on defaultAlarm. Run with -race for this test to catch issues.
func TestInitConcurrent(t *testing.T) {
	t.Cleanup(func() { defaultAlarm.Store(nil) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Initializer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			Init("env", srv.URL)
		}
	}()

	// Sender
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = Send(ctx, "hello")
		}
	}()

	wg.Wait()
}

// TestSenderInterface verifies that *Alarm satisfies the Sender interface.
func TestSenderInterface(t *testing.T) {
	var _ Sender = (*Alarm)(nil)
}
