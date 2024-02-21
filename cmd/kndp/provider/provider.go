package provider

type Cmd struct {
	Install installCmd `cmd:"" help:"Install Crossplane Provider."`
}
