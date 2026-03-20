package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/joho/godotenv"
	grpcserver "github.com/jyotil-raval/mal-updater/internal/grpc"
	"github.com/jyotil-raval/mal-updater/internal/session"
	pb "github.com/jyotil-raval/mal-updater/proto/animepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	godotenv.Load()

	if os.Getenv("MAL_CLIENT_ID") == "" {
		log.Fatal("MAL_CLIENT_ID is not set in .env")
	}

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "9090"
	}

	tok, err := session.LoadOrRefresh()
	if err != nil {
		log.Fatalf("authentication failed: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	animeServer := grpcserver.NewAnimeServer(tok.AccessToken)
	pb.RegisterAnimeServiceServer(grpcServer, animeServer)
	reflection.Register(grpcServer)

	fmt.Printf("gRPC server running on :%s\n", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
