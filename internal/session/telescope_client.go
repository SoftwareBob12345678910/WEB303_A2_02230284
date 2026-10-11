package session

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	telescopepb "stargazer/gen/telescope/v1"
)

// ResilienceConfig groups the tunables of the Service A -> Service B call.
// All values come from environment variables in main.go.
type ResilienceConfig struct {
	Timeout          time.Duration // per-attempt deadline
	MaxRetries       int           // retries AFTER the first attempt
	BaseBackoff      time.Duration // first wait; doubles each retry
	FailureThreshold uint32        // consecutive failures that open the breaker
	OpenDuration     time.Duration // how long the breaker stays open
}

// TelescopeClient wraps the generated gRPC client with the three resilience
// patterns required by the assignment:
//
//	retry (with exponential backoff + jitter)
//	  -> circuit breaker (measures every single attempt)
//	       -> timeout (context deadline per attempt)
//
// Only *transient* errors (Unavailable, DeadlineExceeded) are retried and
// counted as breaker failures. Business answers such as NotFound are valid
// responses from a healthy service and must never trip the breaker.
type TelescopeClient struct {
	client  telescopepb.TelescopeServiceClient
	breaker *gobreaker.CircuitBreaker
	cfg     ResilienceConfig
}

func NewTelescopeClient(client telescopepb.TelescopeServiceClient, cfg ResilienceConfig) *TelescopeClient {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "telescope-service",
		MaxRequests: 1, // half-open: let one probe request through
		Timeout:     cfg.OpenDuration,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= cfg.FailureThreshold
		},
		IsSuccessful: func(err error) bool {
			return err == nil || !isTransient(err)
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("circuit breaker %q: %s -> %s", name, from, to)
		},
	})
	return &TelescopeClient{client: client, breaker: cb, cfg: cfg}
}

// GetTelescope fetches a telescope from Service B with timeout, retry and
// circuit breaker applied. Non-transient errors (e.g. NotFound) are returned
// unchanged; exhausted retries or an open breaker yield codes.Unavailable.
func (c *TelescopeClient) GetTelescope(ctx context.Context, id string) (*telescopepb.Telescope, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			wait := backoff(c.cfg.BaseBackoff, attempt)
			log.Printf("retrying telescope-service (attempt %d/%d) in %s: %v", attempt+1, c.cfg.MaxRetries+1, wait, lastErr)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, status.FromContextError(ctx.Err()).Err()
			}
		}

		res, err := c.breaker.Execute(func() (interface{}, error) {
			callCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout) // TIMEOUT
			defer cancel()
			return c.client.GetTelescope(callCtx, &telescopepb.GetTelescopeRequest{Id: id})
		})
		if err == nil {
			return res.(*telescopepb.Telescope), nil
		}
		lastErr = err

		// CIRCUIT BREAKER open: fail fast, do not waste time retrying.
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			log.Printf("circuit breaker open: skipping call to telescope-service")
			return nil, status.Error(codes.Unavailable, "telescope service is temporarily unavailable (circuit open)")
		}
		// Business error (NotFound, InvalidArgument...): retrying cannot help.
		if !isTransient(err) {
			return nil, err
		}
	}
	return nil, status.Errorf(codes.Unavailable, "telescope service unavailable after %d attempts: %v", c.cfg.MaxRetries+1, lastErr)
}

// isTransient reports whether an error is worth retrying.
func isTransient(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	}
	return false
}

// backoff returns base * 2^(attempt-1) plus up to 25% random jitter, so many
// callers do not retry in lockstep.
func backoff(base time.Duration, attempt int) time.Duration {
	d := base << (attempt - 1)
	return d + time.Duration(rand.Int63n(int64(d)/4+1))
}
