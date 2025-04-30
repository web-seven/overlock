package main

import (
	"fmt"
	"reflect"

	"github.com/web-seven/overlock/plugins/solana/cmd/billing"
	solana "github.com/web-seven/overlock/plugins/solana/cmd/network"

	"github.com/iancoleman/strcase"

	"github.com/alecthomas/kong"
)

type CLI struct {
	Solana struct {
		Network solana.Cmd  `cmd:"" help:"Network information"`
		Billing billing.Cmd `cmd:"" help:"Billing for the Solana environment"`
	} `cmd:"" help:"Solana plugin commands"`
}

func RegisterCommands() []kong.Option {

	cli := CLI{}
	var options []kong.Option
	cliType := reflect.TypeOf(cli)
	cliValue := reflect.ValueOf(&cli).Elem()
	for i := 0; i < cliType.NumField(); i++ {
		field := cliType.Field(i)
		cmdName := strcase.ToKebab(field.Name)
		helpText := field.Tag.Get("help")
		if helpText == "" {
			helpText = fmt.Sprintf("%s command", field.Name)
		}
		options = append(options, kong.DynamicCommand(cmdName, helpText, "", cliValue.Field(i).Addr().Interface()))
	}

	return options
}
