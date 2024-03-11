package provider

type Cmd struct {
	Install installCmd `cmd:"" help:"Install Crossplane Provider."`
	List    listCmd    `cmd:"" help:"Install Crossplane Provider."`
}
