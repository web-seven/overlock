package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/cmd/kndp/configuration"
	"github.com/kndpio/kndp/cmd/kndp/environment"
	"github.com/kndpio/kndp/cmd/kndp/provider"

	"github.com/kndpio/kndp/cmd/kndp/registry"
	"github.com/kndpio/kndp/cmd/kndp/resource"
	"github.com/willabides/kongplete"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Globals struct {
	Debug   bool        `short:"D" help:"Enable debug mode"`
	Version VersionFlag `name:"version" help:"Print version information and quit"`
}

type VersionFlag string

var Version = "development"

func (v VersionFlag) Decode(ctx *kong.DecodeContext) error { return nil }
func (v VersionFlag) IsBool() bool                         { return true }
func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	log.Print(vars["version"])
	app.Exit(0)
	return nil
}

func getDescriptionText() string {
	bText := "Kubernetes Native Development Platform CLI.\n\n"
	bText += "For more details open https://kndp.io \n\n"
	return bText
}

func (c *cli) AfterApply(ctx *kong.Context) error { //nolint:unparam
	config, _ := ctrl.GetConfig()
	if config != nil {
		ctx.Bind(config)
		dynamicClient, _ := dynamic.NewForConfig(config)
		kubeClient, _ := kubernetes.NewForConfig(config)
		ctx.Bind(dynamicClient)
		ctx.Bind(kubeClient)
	}
	logger := log.New(os.Stdout)
	if c.Globals.Debug {
		logger.SetLevel(log.DebugLevel)
	}
	logger.SetReportTimestamp(false)
	ctx.Bind(logger)
	return nil
}

type cli struct {
	Globals

	Help               helpCmd                      `cmd:"" help:"Show help."`
	Environment        environment.Cmd              `cmd:"" name:"environment" aliases:"env" help:"KNDP Environment commands"`
	Configuration      configuration.Cmd            `cmd:"" name:"configuration" aliases:"cfg" help:"KNDP Configuration commands"`
	Resource           resource.Cmd                 `cmd:"" name:"resource" aliases:"res" help:"KNDP Resource commands"`
	Registry           registry.Cmd                 `cmd:"" name:"registry" aliases:"reg" help:"Packages registy commands"`
	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`
	Provider           provider.Cmd                 `cmd:"" name:"provider" help:"KNDP Provider commands"`
}

type helpCmd struct{}

func main() {

	c := cli{
		Globals: Globals{
			Version: VersionFlag(Version),
		},
	}

	parser := kong.Must(&c,
		kong.Name("kndp"),
		kong.Description(getDescriptionText()),
		kong.Help(func(options kong.HelpOptions, ctx *kong.Context) error {
			return kong.DefaultHelpPrinter(options, ctx)
		}),
		kong.Vars{
			"version": Version,
		},
		kong.ConfigureHelp(kong.HelpOptions{
			Tree: true,
		}))

	if len(os.Args) == 1 {
		_, err := parser.Parse([]string{"--help"})
		parser.FatalIfErrorf(err)
		return
	}

	kongCtx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		defer cancel()
		<-sigCh
		kongCtx.Exit(1)
	}()

	kongCtx.BindTo(ctx, (*context.Context)(nil))
	kongCtx.FatalIfErrorf(kongCtx.Run())
}
