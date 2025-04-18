package billing

import (
	"github.com/web-seven/overlock/plugins/cosmos/pkg/billing"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type watchCmd struct {
	Url string ` short:"u" help:"Specifies the url  to connect to."`
}

func (c *watchCmd) Run(client *kubernetes.Clientset, config *rest.Config, dc *dynamic.DynamicClient) error {
	billing.Watch(c.Url)

	return nil
}
