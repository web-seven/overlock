package billing

type Cmd struct {
	Watch watchCmd `cmd:"" help:"Watch billing events"`
}
