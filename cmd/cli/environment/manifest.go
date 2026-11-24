package environment

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/web-seven/overlock/api/v1alpha1"
	overlockerrors "github.com/web-seven/overlock/pkg/errors"
)

// defaultMaxReconcileRate mirrors the default of the create --max-reconcile-rate flag.
const defaultMaxReconcileRate = 1

type manifestCmd struct {
	Name   string `arg:"" optional:"" help:"Name of the Environment resource. If omitted, falls back to 'name' in the Overlock configuration file."`
	Config string `optional:"" help:"Path to the Overlock configuration file. Defaults to ./overlock.yaml if present."`
}

// environmentManifest is an Environment resource without status and server
// managed metadata, for clean YAML output.
type environmentManifest struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec v1alpha1.EnvironmentSpec `json:"spec"`
}

// Run converts the environment part of the Overlock configuration file into an
// Environment custom resource and prints it. CLI-specific settings (engine,
// ports, mounts, nodes) are not part of the resource.
func (c *manifestCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	create := &createCmd{
		Name:          c.Name,
		Config:        c.Config,
		createOptions: createOptions{MaxReconcileRate: defaultMaxReconcileRate},
	}
	stop, err := create.loadAndMergeConfig(logger)
	if err != nil || stop {
		return err
	}

	name := create.Name
	if name == "" {
		name = create.ConfigName
	}
	if name == "" {
		return overlockerrors.NewInvalidConfigError("name", "", "environment name must be provided either as a positional argument or via 'name' in the configuration file")
	}

	spec, err := create.environmentSpec()
	if err != nil {
		return err
	}

	out, err := renderManifest(name, spec)
	if err != nil {
		return err
	}
	fmt.Print(string(out))
	return nil
}

func renderManifest(name string, spec v1alpha1.EnvironmentSpec) ([]byte, error) {
	m := environmentManifest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       v1alpha1.EnvironmentKind,
		},
		Spec: spec,
	}
	m.Metadata.Name = name
	return k8syaml.Marshal(m)
}
