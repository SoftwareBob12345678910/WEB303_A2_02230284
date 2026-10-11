package grpcutil

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial creates a lazily-connecting client connection.
//
// Why these choices:
//   - "passthrough:///" makes Go's dialer resolve the hostname on every
//     reconnect, so when a Docker container restarts with a new IP we pick it
//     up straight away (the default DNS resolver caches for much longer).
//   - A short reconnect backoff (max 3s) means the connection recovers quickly
//     after the target service comes back, which lets the circuit breaker
//     close again soon after recovery.
//   - NewClient does not connect immediately, so start-up order never matters.
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient("passthrough:///"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  500 * time.Millisecond,
				Multiplier: 1.3,
				Jitter:     0.2,
				MaxDelay:   3 * time.Second,
			},
			MinConnectTimeout: 2 * time.Second,
		}),
	)
}
