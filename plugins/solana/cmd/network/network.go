package solana

type Cmd struct {
	Subscribe subscribeCmd `cmd:"" help:"Subscribe to the Environment create event"`
}
