package client

import (
	"log"

	crossplanev1beta1 "github.com/web-seven/overlock-api/go/node/overlock/crossplane/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewClient(target string) (crossplanev1beta1.QueryClient, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to create gRPC connection: %v", err)
		return nil, err
	}

	client := crossplanev1beta1.NewQueryClient(conn)
	return client, nil
}
