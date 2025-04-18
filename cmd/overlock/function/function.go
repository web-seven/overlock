package function

type Cmd struct {
	Apply  applyCmd  `cmd:"" help:"Apply Crossplane Function."`
	List   listCmd   `cmd:"" help:"Apply Crossplane Function."`
	Load   loadCmd   `cmd:"" help:"Load Crossplane Function from archive."`
	Serve  serveCmd  `cmd:"" help:"Watch changes of Function, build and load."`
	Delete deleteCmd `cmd:"" help:"Delete Crossplane Function."`
}
