// Command gateway runs the REST API gateway: the only service exposed to
// clients such as Postman. Everything behind it speaks gRPC.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sessionpb "stargazer/gen/session/v1"
	telescopepb "stargazer/gen/telescope/v1"
	"stargazer/internal/gateway"
	"stargazer/internal/platform/config"
	"stargazer/internal/platform/grpcutil"
)

func main() {
	port := config.MustGet("GATEWAY_PORT")
	telescopeAddr := config.MustGet("TELESCOPE_SERVICE_ADDR")
	sessionAddr := config.MustGet("SESSION_SERVICE_ADDR")
	// Must be longer than Service A's worst case (retries x timeout + backoff).
	timeout := config.Duration("REQUEST_TIMEOUT", 10*time.Second)

	telescopeConn, err := grpcutil.Dial(telescopeAddr)
	if err != nil {
		log.Fatalf("telescope client: %v", err)
	}
	defer telescopeConn.Close()

	sessionConn, err := grpcutil.Dial(sessionAddr)
	if err != nil {
		log.Fatalf("session client: %v", err)
	}
	defer sessionConn.Close()

	gw := gateway.New(
		telescopepb.NewTelescopeServiceClient(telescopeConn),
		sessionpb.NewSessionServiceClient(sessionConn),
		timeout,
	)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           gw.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Println("gateway: shutting down gracefully")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("gateway listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}
