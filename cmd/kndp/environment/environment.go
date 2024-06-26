package environment

type Cmd struct {
	Create  createCmd  `cmd:"" help:"Create an Environment"`
	Delete  deleteCmd  `cmd:"" help:"Delete an Environment"`
	Copy    copyCmd    `cmd:"" help:"Copy an Environment to another destination context"`
	List    listCmd    `cmd:"" help:"List of Environments"`
	Stop    stopCmd    `cmd:"" help:"Stop an Environment"`
	Start   startCmd   `cmd:"" help:"Start an Environment"`
	Upgrade upgradeCmd `cmd:"" help:"Upgrade specified environment context with the latest engine"`
}
