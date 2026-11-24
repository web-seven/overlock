package provider

type Cmd struct {
	Install installCmd `cmd:"" help:"Install Crossplane Provider."`
	List    listCmd    `cmd:"" help:"List all Crossplane Providers."`
	Load    loadCmd    `cmd:"" help:"Load Crossplane Provider."`
	Serve   serveCmd   `cmd:"" help:"Watch changes of Provider, build and load."`
	Delete  deleteCmd  `cmd:"" help:"Delete Crossplane Provider."`
}
