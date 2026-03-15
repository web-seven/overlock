package solana

import (
	"strconv"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"

	"github.com/web-seven/overlock/plugins/solana/pkg/network"
)

type subscribeCmd struct {
	Engine      string `optional:"" short:"e" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"k3s"`
	Creator     string `optional:"" short:"c" help:"Specifies the creator of the environment."`
	Host        string `optional:"" short:"h" help:"Specifies the host address to connect to." default:"0.0.0.0"`
	Port        string `optional:"" short:"p" help:"Specifies the port to connect to." default:"8900"`
	Path        string `optional:"" short:"P" help:"Specifies the path to connect to."  default:"/websocket"`
	GrpcAddress string `optional:"" short:"g" help:"Specifies the gRPC address to connect to." default:"http://127.0.0.1:8899"`

	ProviderName    string `arg:"" required:"" help:"The name of the provider to register."`
	ProviderIP      string `arg:"" required:"" help:"The IP address of the provider."`
	ProviderPort    string `arg:"" required:"" help:"The port of the provider service."`
	CountryCode     string `arg:"" required:"" help:"The country code where the provider is located (e.g., US, DE)."`
	EnvironmentType string `arg:"" required:"" help:"The environment type of the provider (e.g., crossplane, argocd)."`
	Availability    string `arg:"" required:"" help:"Current availability status (e.g., available, maintenance)."`

	ProgramId string `arg:"" required:"" help:"The program ID of the Solana provider."`
	KeyPath   string `arg:"" required:"" help:"The path to the key file for the Solana provider."`
}

func (c *subscribeCmd) Run(client *kubernetes.Clientset, config *rest.Config, dc *dynamic.DynamicClient) error {
	parseUint32 := func(s string) uint32 {
		val, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return 0
		}
		return uint32(val)
	}

	providerMetadata := crossplanev1beta1.Metadata{
		Name: c.ProviderName,
	}

	provider := crossplanev1beta1.MsgCreateProvider{
		Metadata:        &providerMetadata,
		Ip:              c.ProviderIP,
		Port:            parseUint32(c.ProviderPort),
		CountryCode:     c.CountryCode,
		EnvironmentType: c.EnvironmentType,
		Availability:    c.Availability,
	}

	network.Subscribe(c.Engine, c.Creator, c.Host, c.Port, c.Path, c.GrpcAddress, client, config, dc, provider, c.ProgramId, c.KeyPath)
	return nil
}
