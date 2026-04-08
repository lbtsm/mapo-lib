package alarm

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	// reset for other tests
	defaultAlarm = nil
}
