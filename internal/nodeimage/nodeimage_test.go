package nodeimage

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/random"
)

func TestOCIArchiveKeepsDigestAndNames(t *testing.T) {
	img, err := random.Image(1024, 2)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := img.Digest()
	rawManifest, _ := img.RawManifest()
	names := []string{"registry.overlock.svc.cluster.local/acme/provider:v1", "localhost:30100/acme/provider:v1"}

	archive, err := ociArchive(img, names)
	if err != nil {
		t.Fatalf("ociArchive() unexpected error: %v", err)
	}

	files := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(tr)
		files[h.Name] = data
	}

	if _, ok := files["oci-layout"]; !ok {
		t.Error("archive has no oci-layout file")
	}
	if got := files["blobs/sha256/"+digest.Hex]; !bytes.Equal(got, rawManifest) {
		t.Error("manifest blob differs from the registry manifest")
	}

	var index struct {
		Manifests []struct {
			Digest      string            `json:"digest"`
			Annotations map[string]string `json:"annotations"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(files["index.json"], &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Manifests) != len(names) {
		t.Fatalf("index has %d manifests, want %d", len(index.Manifests), len(names))
	}
	for i, m := range index.Manifests {
		if m.Digest != digest.String() || m.Annotations[containerdNameAnnotation] != names[i] {
			t.Errorf("manifest %d = %s %v, want %s named %s", i, m.Digest, m.Annotations, digest, names[i])
		}
	}
}

func TestArchiveName(t *testing.T) {
	a := archiveName([]string{"registry.local/acme/provider:v1", "localhost:30100/acme/provider:v1"})
	if a != archiveName([]string{"registry.local/acme/provider:v1", "localhost:30100/acme/provider:v1"}) {
		t.Error("archive name is not stable, reloading would not replace the archive")
	}
	if a == archiveName([]string{"registry.local/acme/provider:v2"}) {
		t.Error("different images share an archive name")
	}
	if VolumeName("dev") != "overlock-images-dev" {
		t.Errorf("VolumeName() = %q", VolumeName("dev"))
	}
}

func TestFileArchiveHasModTime(t *testing.T) {
	data, err := fileArchive("a.tar", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := tar.NewReader(bytes.NewReader(data)).Next()
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(h.ModTime) > time.Minute {
		t.Errorf("archive modification time = %v, K3s would not re-import a reloaded image of the same size", h.ModTime)
	}
}
