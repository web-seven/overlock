// Package config embeds the operator deployment manifests (kustomize), so the
// CLI can bootstrap the operator matching its own version.
package config

import "embed"

// FS holds the kustomize tree of the operator; config/default is its root.
//
//go:embed default/kustomization.yaml crd/kustomization.yaml crd/bases/*.yaml rbac/*.yaml manager/*.yaml
var FS embed.FS
