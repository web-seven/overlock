package environment

type Cmd struct {
	Create createCmd `cmd:"" help:"Create an Environment"`
	Delete deleteCmd `cmd:"" help:"Delete an Environment"`
	Move   moveCmd   `cmd:"" help:"Move an Environment to another Environemnt"`
	List   listCmd   `cmd:"" help:"List of Environments"`
}
