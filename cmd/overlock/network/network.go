package network

type Cmd struct {
	Subscribe subscribeCmd `cmd:"" aliases:"sub" help:"Apply Crossplane Function."`
}
