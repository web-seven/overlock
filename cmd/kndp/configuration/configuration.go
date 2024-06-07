package configuration

type Cmd struct {
	Apply  applyCmd  `cmd:"" help:"Apply Crossplane Configuration."`
	List   listCmd   `cmd:"" help:"Apply Crossplane Configuration."`
	Load   loadCmd   `cmd:"" help:"Load Crossplane Configuration from archive."`
	Delete deleteCmd `cmd:"" help:"Delete Crossplane Configuration."`
}
