package configuration

type Cmd struct {
	Apply applyCmd `cmd:"" help:"Apply Crossplane Configuration."`
	List  listCmd  `cmd:"" help:"Apply Crossplane Configuration."`
}
