package configuration

type Cmd struct {
	Apply applyCmd `cmd:"" help:"Apply Crossplane Configuration."`
}
