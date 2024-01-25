package environment

type Cmd struct {
	Create createCmd `cmd:"" help:"Create an Environment"`
	Delete deleteCmd `cmd:"" help:"Delete an Environment"`
	Move   moveCmd   `cmd:"" help:"Delete an Environment to another Environemnt"`
}
