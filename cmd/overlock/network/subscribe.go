package network

import (
	"context"

	"github.com/web-seven/overlock/internal/network"
)

type subscribeCmd struct {
	RpcEndpoint string `default:"127.0.0.1:26657" arg:"" help:"Address of network RPC endpoint"`
	Creators    string `default:"" arg:"" help:"List of creators addresses to subscribe"`
}

func (c *subscribeCmd) Run(ctx context.Context) error {
	return network.Subscribe(ctx, c.RpcEndpoint)
}
