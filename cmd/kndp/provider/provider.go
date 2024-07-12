package provider

type Cmd struct {
	Install installCmd `cmd:"" help:"Install Crossplane Provider."`
	Load    loadCmd    `cmd:"" help:"Load Crossplane Provider."`
	List    listCmd    `cmd:"" help:"List all Crossplane Providers."`
	Delete  deleteCmd  `cmd:"" help:"Delete Crossplane Provider."`
	Apply   applyCmd   `cmd:"" help:"Apply Crossplane Provider."`
}
