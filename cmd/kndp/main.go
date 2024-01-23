package main

import (
	"github.com/alecthomas/kong"
	"github.com/kndpio/kndp/cmd/kndp/environment"
	"github.com/willabides/kongplete"
)

type versionFlag bool

type cli struct {
	Version versionFlag `short:"v" name:"version" help:"Print version and exit."`

	Help               helpCmd                      `cmd:"" help:"Show help."`
	Environment        environment.Cmd              `cmd:"" name:"environment" aliases:"cfg" help:"Interact with configurations."`
	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`
}

type helpCmd struct{}

func main() {
	cli := cli{}
	kong.Parse(&cli)
}
