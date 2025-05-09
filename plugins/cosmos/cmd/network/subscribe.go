package cosmos

import (
	"strconv"

	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"
	"github.com/web-seven/overlock/plugins/cosmos/pkg/network"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type subscribeCmd struct {
	Engine      string `optional:"" short:"e" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"k3s"`
	Creator     string `optional:"" short:"c" help:"Specifies the creator of the environment."`
	Host        string `optional:"" short:"h" help:"Specifies the host address to connect to." default:"0.0.0.0"`
	Port        string `optional:"" short:"p" help:"Specifies the port to connect to." default:"26657"`
	Path        string `optional:"" short:"P" help:"Specifies the path to connect to."  default:"/websocket"`
	GrpcAddress string `optional:"" short:"g" help:"Specifies the gRPC address to connect to." default:"localhost:9090"`

	ProviderName    string `arg:"" requried:"" help:"The name of the provider to register."`
	ProviderIP      string `arg:"" requried:"" help:"The IP address of the provider."`
	ProviderPort    string `arg:"" requried:"" help:"The port of the provider service."`
	CountryCode     string `arg:"" requried:"" help:"The country code where the provider is located (e.g., US, DE)."`
	EnvironmentType string `arg:"" requried:"" help:"The environment type of the provider (e.g., crossplane, argocd)."`
	Availability    string `arg:"" requried:"" help:"Current availability status (e.g., available, maintenance)."`

	ChainID        string `help:"Chain ID of the Cosmos SDK chain." default:"overlock"`
	ImportKeyName  string `arg:"" requried:"" help:"Name of the key."`
	ImportKeyPath  string `arg:"" requried:"" help:"Path of the key to import into keyring."`
	KeyringBackend string `arg:"" requried:"" help:"Keyring backend to use (e.g., file, os, kwallet, pass, test, memory)."`
}

func (c *subscribeCmd) Run(clientset *kubernetes.Clientset, config *rest.Config, dc *dynamic.DynamicClient) error {

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
	network.Subscribe(c.Engine, c.Creator, c.Host, c.Port, c.Path, c.GrpcAddress, clientset, config, dc, provider, c.ImportKeyName, c.ImportKeyPath, c.ChainID, c.KeyringBackend)
	return nil
}
