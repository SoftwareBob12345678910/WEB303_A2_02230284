package session

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	telescopepb "stargazer/gen/telescope/v1"
)

// fakeTelescopes embeds the generated client interface and overrides only the
// one method the resilience wrapper uses.
type fakeTelescopes struct {
	telescopepb.TelescopeServiceClient
	calls int
	err   error
}

func (f *fakeTelescopes) GetTelescope(_ context.Context, in *telescopepb.GetTelescopeRequest, _ ...grpc.CallOption) (*telescopepb.Telescope, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &telescopepb.Telescope{Id: in.GetId(), Name: "Test scope", Status: "AVAILABLE"}, nil
}

func testConfig() ResilienceConfig {
	return ResilienceConfig{
		Timeout:          50 * time.Millisecond,
		MaxRetries:       2,
		BaseBackoff:      time.Millisecond,
		FailureThreshold: 5,
		OpenDuration:     time.Hour, // stay open for the whole test
	}
}

func TestSuccessNoRetry(t *testing.T) {
	f := &fakeTelescopes{}
	c := NewTelescopeClient(f, testConfig())
	tel, err := c.GetTelescope(context.Background(), "id-1")
	if err != nil || tel.GetName() != "Test scope" {
		t.Fatalf("unexpected result: %v %v", tel, err)
	}
	if f.calls != 1 {
		t.Fatalf("want 1 call, got %d", f.calls)
	}
}

func TestRetriesTransientErrors(t *testing.T) {
	f := &fakeTelescopes{err: status.Error(codes.Unavailable, "down")}
	c := NewTelescopeClient(f, testConfig())
	_, err := c.GetTelescope(context.Background(), "id-1")
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("want Unavailable, got %v", err)
	}
	if f.calls != 3 { // 1 attempt + 2 retries
		t.Fatalf("want 3 attempts, got %d", f.calls)
	}
}

func TestDoesNotRetryNotFound(t *testing.T) {
	f := &fakeTelescopes{err: status.Error(codes.NotFound, "no such telescope")}
	c := NewTelescopeClient(f, testConfig())
	_, err := c.GetTelescope(context.Background(), "id-1")
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("NotFound must not be retried, got %d calls", f.calls)
	}
}

func TestCircuitBreakerOpensAndFailsFast(t *testing.T) {
	f := &fakeTelescopes{err: status.Error(codes.Unavailable, "down")}
	c := NewTelescopeClient(f, testConfig())

	// Call 1: 3 failed attempts. Call 2: 2 more failures trip the breaker
	// (5 consecutive), and its third attempt is rejected without a real call.
	for i := 0; i < 2; i++ {
		_, _ = c.GetTelescope(context.Background(), "id-1")
	}
	if f.calls != 5 {
		t.Fatalf("want 5 real calls before the breaker opens, got %d", f.calls)
	}

	// Call 3: breaker is open -> fails immediately, no call reaches Service B.
	_, err := c.GetTelescope(context.Background(), "id-1")
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("want Unavailable, got %v", err)
	}
	if f.calls != 5 {
		t.Fatalf("open breaker must not call the service, got %d calls", f.calls)
	}
}

func TestNotFoundNeverTripsBreaker(t *testing.T) {
	f := &fakeTelescopes{err: status.Error(codes.NotFound, "missing")}
	c := NewTelescopeClient(f, testConfig())
	for i := 0; i < 20; i++ {
		_, _ = c.GetTelescope(context.Background(), "id-1")
	}
	if f.calls != 20 {
		t.Fatalf("breaker must stay closed for NotFound, got %d calls", f.calls)
	}
}
