// Package grpcutil holds the gRPC plumbing shared by every service:
// server bootstrap (logging, panic recovery, health, graceful shutdown)
// and a client dialer tuned for fast failure and fast recovery.
package grpcutil

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Run starts a gRPC server on the given port and blocks until SIGINT/SIGTERM,
// then drains in-flight requests (graceful shutdown, important in Docker).
func Run(name, port string, register func(s *grpc.Server)) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(recovery(), logging()))
	register(srv)

	hs := health.NewServer()
	healthpb.RegisterHealthServer(srv, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(srv) // lets grpcurl discover methods without .proto files

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Printf("%s: shutting down gracefully", name)
		srv.GracefulStop()
	}()

	log.Printf("%s listening on :%s", name, port)
	return srv.Serve(lis)
}

// logging prints one line per RPC: method, resulting gRPC code and duration.
// These lines are handy evidence in the report.
func logging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := h(ctx, req)
		log.Printf("grpc method=%s code=%s duration=%s", info.FullMethod, status.Code(err), time.Since(start).Round(time.Millisecond))
		return resp, err
	}
}

// recovery turns a panic inside a handler into a clean Internal error
// instead of crashing the whole service.
func recovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in %s: %v\n%s", info.FullMethod, r, debug.Stack())
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return h(ctx, req)
	}
}
