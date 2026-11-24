package resource

type Cmd struct {
	Create createCmd `cmd:"" help:"Create an XR"`
	List   listCmd   `cmd:"" help:"List of XRs"`
	Apply  applyCmd  `cmd:"" help:"Apply an XR"`
}
