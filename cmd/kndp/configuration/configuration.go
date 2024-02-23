package configuration

type Cmd struct {
	Apply applyCmd `cmd:"" help:"Apply Crossplane Configuration."`
	List  listCmd  `cmd:"" help:"Apply Crossplane Configuration."`
	Delete deleteCmd `cmd:"" help:"Delete Crossplane Configuration."`
}
