package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const (
	// LabelEnvironment marks a resource or Helm release as owned by the named
	// Environment. Resources without it are never modified or removed.
	LabelEnvironment = "overlock.app/environment"
	// labelSpecHash stores the hash of the desired chart version and values
	// on an owned Helm release, to detect when an upgrade is needed.
	labelSpecHash = "overlock.app/spec-hash"
	// labelComponent identifies the component a resource belongs to.
	labelComponent = "app.kubernetes.io/component"
	labelManagedBy = "app.kubernetes.io/managed-by"
	managedBy      = "overlock"

	// finalizer guarantees the deletion policy is applied before an
	// Environment is removed.
	finalizer = "overlock.app/environment"
)

// ownerLabels returns the labels marking a resource as owned by env.
func ownerLabels(env string) map[string]string {
	return map[string]string{
		LabelEnvironment: env,
		labelManagedBy:   managedBy,
	}
}

// isOwnedBy reports whether labels mark a resource as owned by env.
func isOwnedBy(labels map[string]string, env string) bool {
	return labels[LabelEnvironment] == env
}

// isOwnedByOther reports whether labels mark a resource as owned by another
// Environment.
func isOwnedByOther(labels map[string]string, env string) bool {
	owner, ok := labels[LabelEnvironment]
	return ok && owner != env
}

// specHash returns a short stable hash of a chart version and values.
func specHash(version string, values map[string]any) string {
	data, _ := json.Marshal(struct {
		Version string         `json:"version"`
		Values  map[string]any `json:"values"`
	}{version, values})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
}
