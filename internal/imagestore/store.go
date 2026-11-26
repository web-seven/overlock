// Package imagestore serves images stored in a node's containerd through the
// read-only part of the OCI distribution API, so Crossplane can install
// packages from containerd instead of a separate registry. Images of upstream
// registries are pulled into containerd on first use (pull-through cache).
package imagestore

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ErrNotFound is returned by a Store for unknown images and content.
var ErrNotFound = errors.New("not found")

// Store is the image storage the server reads from.
type Store interface {
	// Resolve returns the descriptor an image name with tag points to.
	Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error)
	// Size returns the size of content.
	Size(ctx context.Context, dgst digest.Digest) (int64, error)
	// Open returns a reader for content.
	Open(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error)
	// Tags returns the tags of the images named name (without tag).
	Tags(ctx context.Context, name string) ([]string, error)
	// Pull fetches an image (name with tag or digest) into the store.
	Pull(ctx context.Context, ref string) error
}

// imageName maps a repository requested from the store to the name of the
// image in containerd. A repository starting with a registry host refers to
// that registry ("xpkg.crossplane.io/crossplane-contrib/provider-nop"); any
// other repository refers to the local registry domain, where packages loaded
// by the CLI are stored ("acme/nop" -> "<local>/acme/nop"). The boolean is
// true for upstream repositories, which are pulled when missing.
func imageName(repo, localDomain string) (string, bool) {
	first, _, found := strings.Cut(repo, "/")
	if found && (strings.ContainsAny(first, ".:") || first == "localhost") {
		return repo, true
	}
	return localDomain + "/" + repo, false
}
