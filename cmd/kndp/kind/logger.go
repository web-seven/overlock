package kind

import (
	"io"
	"os"

	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/env"
)

// NewLogger returns the standard logger used by the kind CLI
// This logger writes to os.Stderr
func NewLogger() log.Logger {
	var writer io.Writer = os.Stderr
	if env.IsSmartTerminal(writer) {
		writer = cli.NewSpinner(writer)
	}
	return cli.NewLogger(writer, 0)
}

// ColorEnabled returns true if color is enabled for the logger
// this should be used to control output
func ColorEnabled(logger log.Logger) bool {
	type maybeColorer interface {
		ColorEnabled() bool
	}
	v, ok := logger.(maybeColorer)
	return ok && v.ColorEnabled()
}
