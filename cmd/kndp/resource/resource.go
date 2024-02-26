package resource

type Cmd struct {
	Create createCmd `cmd:"" help:"Create an XR"`
	List   listCmd   `cmd:"" help:"Create an XR"`
}
