// Package nodeimage imports images into the containerd of local K3s Docker
// nodes, so pods start from them without pulling from a registry.
package nodeimage

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"

	regv1 "github.com/google/go-containerregistry/pkg/v1"
)

// containerdNameAnnotation names an image of an OCI archive on import.
const containerdNameAnnotation = "io.containerd.image.name"

// ociArchive returns an OCI image layout archive of img with one index entry
// per name. The manifest is stored unchanged, so the imported image keeps the
// digest it has in the registry, and its media types (e.g. of Crossplane
// packages) are preserved.
func ociArchive(img regv1.Image, names []string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "overlock-oci-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	path, err := layout.Write(dir, empty.Index)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI layout: %w", err)
	}
	for _, name := range names {
		if err := path.AppendImage(img, layout.WithAnnotations(map[string]string{containerdNameAnnotation: name})); err != nil {
			return nil, fmt.Errorf("failed to add %s to OCI layout: %w", name, err)
		}
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: filepath.ToSlash(rel), Mode: 0o644, Size: info.Size(), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to archive OCI layout: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fileArchive wraps content as a single file named name in a tar stream, the
// format Docker expects to copy files into a container.
func fileArchive(name string, content []byte) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		return nil, err
	}
	if _, err := tw.Write(content); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
