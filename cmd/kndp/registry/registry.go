package registry

type Cmd struct {
	Auth authCmd `cmd:"" help:"Registry Authentication"`
}
