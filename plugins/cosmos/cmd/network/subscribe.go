package cosmos

import (
	"github.com/web-seven/overlock/plugins/cosmos/pkg/network"
)

type subscribeCmd struct {
	Engine  string `optional:"" short:"e" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"k3s"`
	Creator string `optional:"" short:"c" help:"Specifies the creator of the environment."`
	Host    string `optional:"" short:"h" help:"Specifies the host address to connect to." default:"0.0.0.0"`
	Port    string `optional:"" short:"p" help:"Specifies the port to connect to." default:"26657"`
	Path    string `optional:"" short:"P" help:"Specifies the path to connect to."  default:"/websocket"`
}

func (c *subscribeCmd) Run() error {
	network.Subscribe(c.Engine, c.Creator, c.Host, c.Port, c.Path)
	return nil
}
