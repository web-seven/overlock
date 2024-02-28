package registry

type Cmd struct {
	Create createCmd `cmd:"" help:"Create registry"`
	List   listCmd   `cmd:"" help:"List registries"`
}
