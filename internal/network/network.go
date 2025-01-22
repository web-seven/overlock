package network

import (
	"context"
	"fmt"
	"os"
	"time"

	logger "github.com/cometbft/cometbft/libs/log"
	cbftc "github.com/cometbft/cometbft/rpc/jsonrpc/client"
)

func Subscribe(ctx context.Context, endpoint string) error {
	timeout, err := time.ParseDuration("10s")
	if err != nil {
		return err
	}
	c, err := cbftc.NewWS("http://"+endpoint, "/websocket")
	if err != nil {
		return err
	}
	err = c.Start()
	if err != nil {
		return err
	}
	c.SetLogger(logger.NewTMLogger(os.Stdout))

	c.Call(context.Background(), "a", make(map[string]any)) //nolint:errcheck // ignore for tests
	// Let the readRoutine get around to blocking
	time.Sleep(time.Second)
	passCh := make(chan struct{})
	go func() {
		// Unless we have a non-blocking write to ResponsesCh from readRoutine
		// this blocks forever ont the waitgroup
		err := c.Stop()
		if err != nil {
			return
		}
		passCh <- struct{}{}
	}()
	select {
	case <-passCh:
		// Pass
	case <-time.After(timeout):
		return fmt.Errorf("WSClient did failed to stop within %v seconds - is one of the read/write routines blocking?", timeout.Seconds())
	}
	return nil
}
