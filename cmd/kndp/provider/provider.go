package provider

type Cmd struct {
	Install installCmd `cmd:"" help:"Install Crossplane Provider."`
	List    listCmd    `cmd:"" help:"List all Crossplane Providers."`
	Delete  deleteCmd  `cmd:"" help:"Delete Crossplane Provider."`
}
