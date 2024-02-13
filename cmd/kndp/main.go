package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
	"github.com/kndpio/kndp/cmd/kndp/configuration"
	"github.com/kndpio/kndp/cmd/kndp/environment"
	"github.com/kndpio/kndp/cmd/kndp/resource"
	"github.com/pterm/pterm"
	"github.com/willabides/kongplete"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Globals struct {
	Debug   bool        `short:"D" help:"Enable debug mode"`
	Version VersionFlag `name:"version" help:"Print version information and quit"`
}

type VersionFlag string

func (v VersionFlag) Decode(ctx *kong.DecodeContext) error { return nil }
func (v VersionFlag) IsBool() bool                         { return true }
func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Println(vars["version"])
	app.Exit(0)
	return nil
}

func (c *cli) AfterApply(ctx *kong.Context) error { //nolint:unparam
	config := ctrl.GetConfigOrDie()
	dynamicClient := dynamic.NewForConfigOrDie(config)
	ctx.Bind(config)
	ctx.Bind(dynamicClient)
	ctx.BindTo(pterm.DefaultBasicText.WithWriter(ctx.Stdout), (*pterm.TextPrinter)(nil))
	return nil
}

type cli struct {
	Globals

	Help               helpCmd                      `cmd:"" help:"Show help."`
	Environment        environment.Cmd              `cmd:"" name:"environment" aliases:"env" help:"KNDP Environment commands"`
	Configuration      configuration.Cmd            `cmd:"" name:"configuration" aliases:"cfg" help:"KNDP Configuration commands"`
	Resource           resource.Cmd                 `cmd:"" name:"resource" aliases:"res" help:"KNDP Resource commands"`
	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`
}

type helpCmd struct{}

func main() {
	c := cli{
		Globals: Globals{
			Version: VersionFlag("0.0.1"),
		},
	}

	parser := kong.Must(&c,
		kong.Name("kndp"),
		kong.Description("The KNDP CLI"),
		kong.Help(func(options kong.HelpOptions, ctx *kong.Context) error {
			return kong.DefaultHelpPrinter(options, ctx)
		}),
		kong.Vars{
			"version": "0.0.1",
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
