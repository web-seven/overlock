package environment

import (
	"time"

	"github.com/web-seven/overlock/cmd/cli/version"
	"github.com/web-seven/overlock/internal/bootstrap"
	"github.com/web-seven/overlock/internal/engine"
)

// operatorOptions configure how the operator is bootstrapped into the
// environment cluster.
type operatorOptions struct {
	OperatorImage string        `optional:"" help:"Overlock operator image. Defaults to the image matching the CLI version."`
	Wait          time.Duration `optional:"" help:"How long to wait for the environment to become ready. 0 does not wait." default:"15m"`
}

// image returns the operator image to bootstrap.
func (o operatorOptions) image() string {
	if o.OperatorImage != "" {
		return o.OperatorImage
	}
	return bootstrap.Image(version.Version)
}

// engineVersionSet reports whether the Crossplane version was chosen with
// --engine-version or OVERLOCK_ENGINE_VERSION.
func engineVersionSet() bool {
	return engine.Version != engine.DefaultVersion
}
