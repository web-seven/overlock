package cosmos

type Cmd struct {
	Subscribe subscribeCmd `cmd:"" help:"Subscribe to the Msg Environment create event"`
}
