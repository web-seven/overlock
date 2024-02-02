package configuration

import (
	"github.com/alecthomas/kong"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Cmd struct {
	Apply applyCmd `cmd:"" help:"Apply Crossplane Configuration."`
}

func (c *Cmd) AfterApply(kongCtx *kong.Context) error {
	config := ctrl.GetConfigOrDie()
	dynamicClient := dynamic.NewForConfigOrDie(config)
	kongCtx.Bind(config)
	kongCtx.Bind(dynamicClient)
	return nil
}
