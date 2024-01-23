package environment

type Cmd struct {
	Create createCmd `cmd:"" help:"Create a repository."`
	Delete deleteCmd `cmd:"" help:"Delete a repository."`
}
