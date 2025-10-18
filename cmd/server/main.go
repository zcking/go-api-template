package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	userspb "github.com/zcking/go-api-template/gen/go/users/v1"
	"github.com/zcking/go-api-template/internal"
)

var (
	databaseLocation = flag.String("database-location", "lite.duckdb", "Location of the DuckDB database file")
)

func main() {
	// Initialize zap logger
	logger, _ := zap.NewProduction(zap.AddCaller())
	defer logger.Sync()
	flag.Parse()

	// Create a TCP listener for the gRPC server
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to create (gRPC) listener: %v", err)
	}

	// Create a gRPC server and attach our implementation
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_zap.UnaryServerInterceptor(logger),
		)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_ctxtags.StreamServerInterceptor(),
			grpc_zap.StreamServerInterceptor(logger),
		)),
	}
	grpcServer := grpc.NewServer(opts...)
	impl, err := internal.NewUsersServer(*databaseLocation)
	if err != nil {
		log.Fatalf("failed to create UsersServer instance: %v", err)
	}
	userspb.RegisterUserServiceServer(grpcServer, impl)

	// Catch interrupt signal to gracefully shutdown the server
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-signalChan
		log.Printf("received signal %s | shutting down gRPC server...\n", sig)
		grpcServer.GracefulStop()
		if err := impl.Close(); err != nil {
			log.Fatalf("failed to properly close UsersServer: %v", err)
		}
	}()

	// Serve the gRPC server, in a separate goroutine to avoid blocking
	go func() {
		log.Fatalln(grpcServer.Serve(lis))
	}()

	// Now setup the gRPC Gateway, a REST proxy to the gRPC server
	conn, err := grpc.NewClient(
		"0.0.0.0:8080",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to create gRPC client: %v", err)
	}

	mux := runtime.NewServeMux()
	err = userspb.RegisterUserServiceHandler(context.Background(), mux, conn)
	if err != nil {
		log.Fatalf("failed to register gRPC gateway: %v", err)
	}

	// Start HTTP server to proxy requests to gRPC server
	gwServer := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}
	log.Println("gRPC Gateway listening on http://0.0.0.0:8081")
	log.Fatalln(gwServer.ListenAndServe())
}
