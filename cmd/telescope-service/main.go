// Command telescope-service runs Service B: the Telescope gRPC service
// backed by its own PostgreSQL database.
package main

import (
	"context"
	"log"

	"google.golang.org/grpc"

	telescopepb "stargazer/gen/telescope/v1"
	"stargazer/internal/platform/config"
	"stargazer/internal/platform/database"
	"stargazer/internal/platform/grpcutil"
	"stargazer/internal/telescope"
)

func main() {
	ctx := context.Background()

	db, err := database.Connect(ctx, database.DSNFromEnv())
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	repo := telescope.NewRepository(db)
	if err := repo.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	err = grpcutil.Run("telescope-service", config.MustGet("GRPC_PORT"), func(s *grpc.Server) {
		telescopepb.RegisterTelescopeServiceServer(s, telescope.NewServer(repo))
	})
	if err != nil {
		log.Fatalf("serve: %v", err)
	}
}
