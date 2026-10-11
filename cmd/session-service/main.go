// Command session-service runs Service A: the Session gRPC service with its
// own PostgreSQL database. It calls the Telescope service over gRPC with
// timeout, retry and circuit breaker.
package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"

	sessionpb "stargazer/gen/session/v1"
	telescopepb "stargazer/gen/telescope/v1"
	"stargazer/internal/platform/config"
	"stargazer/internal/platform/database"
	"stargazer/internal/platform/grpcutil"
	"stargazer/internal/session"
)

func main() {
	ctx := context.Background()

	db, err := database.Connect(ctx, database.DSNFromEnv())
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	repo := session.NewRepository(db)
	if err := repo.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	conn, err := grpcutil.Dial(config.MustGet("TELESCOPE_SERVICE_ADDR"))
	if err != nil {
		log.Fatalf("create telescope client: %v", err)
	}
	defer conn.Close()

	telescopes := session.NewTelescopeClient(
		telescopepb.NewTelescopeServiceClient(conn),
		session.ResilienceConfig{
			Timeout:          config.Duration("TELESCOPE_TIMEOUT", 2*time.Second),
			MaxRetries:       config.Int("TELESCOPE_MAX_RETRIES", 2),
			BaseBackoff:      config.Duration("TELESCOPE_RETRY_BACKOFF", 200*time.Millisecond),
			FailureThreshold: uint32(config.Int("BREAKER_FAILURE_THRESHOLD", 5)),
			OpenDuration:     config.Duration("BREAKER_OPEN_DURATION", 10*time.Second),
		},
	)

	err = grpcutil.Run("session-service", config.MustGet("GRPC_PORT"), func(s *grpc.Server) {
		sessionpb.RegisterSessionServiceServer(s, session.NewServer(repo, telescopes))
	})
	if err != nil {
		log.Fatalf("serve: %v", err)
	}
}
