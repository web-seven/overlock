package imagestore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const local = "registry.overlock.svc.cluster.local"

// fakeStore is an in-memory Store. upstream holds images Pull can fetch.
type fakeStore struct {
	images   map[string]ocispec.Descriptor
	content  map[digest.Digest][]byte
	upstream map[string]ocispec.Descriptor
	pulls    []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{images: map[string]ocispec.Descriptor{}, content: map[digest.Digest][]byte{}, upstream: map[string]ocispec.Descriptor{}}
}

func (f *fakeStore) add(images map[string]ocispec.Descriptor, ref, mediaType string, manifest, blob []byte) (digest.Digest, digest.Digest) {
	md, bd := digest.FromBytes(manifest), digest.FromBytes(blob)
	images[ref] = ocispec.Descriptor{MediaType: mediaType, Digest: md, Size: int64(len(manifest))}
	f.content[md], f.content[bd] = manifest, blob
	return md, bd
}

func (f *fakeStore) Resolve(_ context.Context, ref string) (ocispec.Descriptor, error) {
	if d, ok := f.images[ref]; ok {
		return d, nil
	}
	return ocispec.Descriptor{}, ErrNotFound
}

func (f *fakeStore) Size(_ context.Context, d digest.Digest) (int64, error) {
	if c, ok := f.content[d]; ok {
		return int64(len(c)), nil
	}
	return 0, ErrNotFound
}

func (f *fakeStore) Open(_ context.Context, d digest.Digest) (io.ReadCloser, error) {
	if c, ok := f.content[d]; ok {
		return io.NopCloser(bytes.NewReader(c)), nil
	}
	return nil, ErrNotFound
}

func (f *fakeStore) Tags(_ context.Context, name string) ([]string, error) {
	var tags []string
	for ref := range f.images {
		if n, tag, ok := strings.Cut(ref, ":"); ok && n == name && !strings.Contains(tag, "/") {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func (f *fakeStore) Pull(_ context.Context, ref string) error {
	f.pulls = append(f.pulls, ref)
	d, ok := f.upstream[ref]
	if !ok {
		return errors.New("upstream: not found")
	}
	f.images[ref] = d
	return nil
}

func do(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestImageName(t *testing.T) {
	for repo, want := range map[string]struct {
		name     string
		upstream bool
	}{
		"acme/nop": {local + "/acme/nop", false},
		"nop":      {local + "/nop", false},
		"xpkg.crossplane.io/crossplane-contrib/x": {"xpkg.crossplane.io/crossplane-contrib/x", true},
		"localhost:5000/team/x":                   {"localhost:5000/team/x", true},
	} {
		name, upstream := imageName(repo, local)
		if name != want.name || upstream != want.upstream {
			t.Errorf("imageName(%q) = %q, %v; want %q, %v", repo, name, upstream, want.name, want.upstream)
		}
	}
}

func TestServeLocalPackage(t *testing.T) {
	f := newFakeStore()
	manifest := []byte(`{"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[]}`)
	md, bd := f.add(f.images, local+"/acme/nop:v1", "application/vnd.oci.image.manifest.v1+json", manifest, []byte("layer"))
	s := &Server{Store: f, LocalDomain: local}

	if rec := do(t, s, http.MethodGet, "/v2/"); rec.Code != http.StatusOK || rec.Header().Get("Docker-Distribution-API-Version") != "registry/2.0" {
		t.Errorf("/v2/ = %d %v", rec.Code, rec.Header())
	}

	head := do(t, s, http.MethodHead, "/v2/acme/nop/manifests/v1")
	if head.Code != http.StatusOK || head.Header().Get("Docker-Content-Digest") != md.String() || head.Body.Len() != 0 {
		t.Errorf("HEAD manifest = %d %v body=%d", head.Code, head.Header(), head.Body.Len())
	}
	get := do(t, s, http.MethodGet, "/v2/acme/nop/manifests/"+md.String())
	if get.Code != http.StatusOK || !bytes.Equal(get.Body.Bytes(), manifest) || get.Header().Get("Content-Type") != "application/vnd.oci.image.manifest.v1+json" {
		t.Errorf("GET manifest by digest = %d %q %v", get.Code, get.Body.String(), get.Header())
	}
	blob := do(t, s, http.MethodGet, "/v2/acme/nop/blobs/"+bd.String())
	if blob.Code != http.StatusOK || blob.Body.String() != "layer" {
		t.Errorf("GET blob = %d %q", blob.Code, blob.Body.String())
	}

	if rec := do(t, s, http.MethodGet, "/v2/acme/missing/manifests/v1"); rec.Code != http.StatusNotFound {
		t.Errorf("missing local manifest = %d, want 404", rec.Code)
	}
	if len(f.pulls) != 0 {
		t.Errorf("local images must never be pulled, pulls = %v", f.pulls)
	}
}

func TestPullThroughUpstream(t *testing.T) {
	f := newFakeStore()
	ref := "xpkg.crossplane.io/acme/config:v2"
	manifest := []byte(`{"mediaType":"application/vnd.docker.distribution.manifest.v2+json"}`)
	md, _ := f.add(f.upstream, ref, "application/vnd.docker.distribution.manifest.v2+json", manifest, []byte("b"))
	s := &Server{Store: f, LocalDomain: local}

	rec := do(t, s, http.MethodGet, "/v2/xpkg.crossplane.io/acme/config/manifests/v2")
	if rec.Code != http.StatusOK || rec.Header().Get("Docker-Content-Digest") != md.String() {
		t.Fatalf("pull-through manifest = %d %s", rec.Code, rec.Body.String())
	}
	if !reflect.DeepEqual(f.pulls, []string{ref}) {
		t.Errorf("pulls = %v, want [%s]", f.pulls, ref)
	}
	do(t, s, http.MethodGet, "/v2/xpkg.crossplane.io/acme/config/manifests/v2")
	if len(f.pulls) != 1 {
		t.Errorf("cached image pulled again, pulls = %v", f.pulls)
	}
	if rec := do(t, s, http.MethodGet, "/v2/xpkg.crossplane.io/acme/config/manifests/v9"); rec.Code != http.StatusBadGateway {
		t.Errorf("unavailable upstream = %d, want 502", rec.Code)
	}
}

func TestTagsList(t *testing.T) {
	f := newFakeStore()
	f.add(f.images, "xpkg.crossplane.io/acme/config:v1.0.0", "x", []byte("m1"), []byte("b1"))
	s := &Server{Store: f, LocalDomain: local, UpstreamTags: func(context.Context, string) ([]string, error) {
		return []string{"v1.0.0", "v1.1.0"}, nil
	}}
	rec := do(t, s, http.MethodGet, "/v2/xpkg.crossplane.io/acme/config/tags/list")
	var got struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("tags/list = %d %s", rec.Code, rec.Body.String())
	}
	if got.Name != "xpkg.crossplane.io/acme/config" || !reflect.DeepEqual(got.Tags, []string{"v1.0.0", "v1.1.0"}) {
		t.Errorf("tags/list = %+v, want cached and upstream tags merged", got)
	}

	offline := &Server{Store: f, LocalDomain: local, UpstreamTags: func(context.Context, string) ([]string, error) {
		return nil, errors.New("offline")
	}}
	rec = do(t, offline, http.MethodGet, "/v2/xpkg.crossplane.io/acme/config/tags/list")
	if !strings.Contains(rec.Body.String(), `"v1.0.0"`) || strings.Contains(rec.Body.String(), "v1.1.0") {
		t.Errorf("offline tags/list = %s, want cached tags only", rec.Body.String())
	}
}

func TestReadOnly(t *testing.T) {
	s := &Server{Store: newFakeStore(), LocalDomain: local}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if rec := do(t, s, m, "/v2/acme/nop/blobs/uploads/"); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s = %d, want 405", m, rec.Code)
		}
	}
}
