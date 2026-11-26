package nodeimage

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

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

func TestImageDigest(t *testing.T) {
	out := `REF                                                   TYPE                                       DIGEST                                                                  SIZE    PLATFORMS   LABELS
localhost:30100/acme/provider:v1                      application/vnd.oci.image.manifest.v1+json sha256:aaa1                                                             17.0 MiB linux/amd64 io.cri-containerd.image=managed
registry.overlock.svc.cluster.local/acme/provider:v1  application/vnd.oci.image.manifest.v1+json sha256:bbb2                                                             17.0 MiB linux/amd64 io.cri-containerd.image=managed
`
	if got := imageDigest(out, "registry.overlock.svc.cluster.local/acme/provider:v1"); got != "sha256:bbb2" {
		t.Errorf("imageDigest() = %q, want sha256:bbb2", got)
	}
	if got := imageDigest(out, "missing:v1"); got != "" {
		t.Errorf("imageDigest() for missing image = %q", got)
	}
}
