package imagestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/opencontainers/go-digest"
	"go.uber.org/zap"
)

// TagLister lists the tags of an upstream repository, so version ranges also
// see versions that are not cached yet.
type TagLister func(ctx context.Context, repo string) ([]string, error)

// Server serves a Store through the OCI distribution API (pull only).
type Server struct {
	Store       Store
	LocalDomain string
	// UpstreamTags lists tags of upstream repositories; nil lists only cached tags.
	UpstreamTags TagLister
	Logger       *zap.SugaredLogger
}

// ServeHTTP implements the OCI distribution pull endpoints:
// /v2/, /v2/<name>/manifests/<ref>, /v2/<name>/blobs/<digest> and
// /v2/<name>/tags/list.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.error(w, http.StatusMethodNotAllowed, "UNSUPPORTED", "the image store is read-only")
		return
	}
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

	path := strings.TrimPrefix(r.URL.Path, "/v2/")
	switch {
	case r.URL.Path == "/v2/" || r.URL.Path == "/v2":
		w.WriteHeader(http.StatusOK)
	case strings.HasSuffix(path, "/tags/list"):
		s.tags(w, r, strings.TrimSuffix(path, "/tags/list"))
	case strings.Contains(path, "/manifests/"):
		repo, ref := cutLast(path, "/manifests/")
		s.manifest(w, r, repo, ref)
	case strings.Contains(path, "/blobs/"):
		_, ref := cutLast(path, "/blobs/")
		s.blob(w, r, ref)
	default:
		s.error(w, http.StatusNotFound, "NAME_UNKNOWN", "unknown endpoint")
	}
}

func (s *Server) manifest(w http.ResponseWriter, r *http.Request, repo, ref string) {
	ctx := r.Context()
	name, upstream := imageName(repo, s.LocalDomain)

	var dgst digest.Digest
	var mediaType string
	if d, err := digest.Parse(ref); err == nil {
		dgst = d
	} else {
		desc, err := s.resolve(ctx, name+":"+ref, upstream)
		if err != nil {
			s.storeError(w, err, "MANIFEST_UNKNOWN", name+":"+ref)
			return
		}
		dgst, mediaType = desc.Digest, desc.MediaType
	}

	data, err := s.read(ctx, dgst, name+"@"+dgst.String(), upstream)
	if err != nil {
		s.storeError(w, err, "MANIFEST_UNKNOWN", name+"@"+dgst.String())
		return
	}
	if mediaType == "" {
		mediaType = manifestMediaType(data)
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Docker-Content-Digest", dgst.String())
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

func (s *Server) blob(w http.ResponseWriter, r *http.Request, ref string) {
	ctx := r.Context()
	dgst, err := digest.Parse(ref)
	if err != nil {
		s.error(w, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
		return
	}
	size, err := s.Store.Size(ctx, dgst)
	if err != nil {
		s.storeError(w, err, "BLOB_UNKNOWN", dgst.String())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", dgst.String())
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	if r.Method == http.MethodHead {
		return
	}
	rc, err := s.Store.Open(ctx, dgst)
	if err != nil {
		s.storeError(w, err, "BLOB_UNKNOWN", dgst.String())
		return
	}
	defer rc.Close()
	if _, err := io.Copy(w, rc); err != nil && s.Logger != nil {
		s.Logger.Debugf("Failed to stream blob %s: %v", dgst, err)
	}
}

func (s *Server) tags(w http.ResponseWriter, r *http.Request, repo string) {
	ctx := r.Context()
	name, upstream := imageName(repo, s.LocalDomain)
	tags, err := s.Store.Tags(ctx, name)
	if err != nil {
		s.storeError(w, err, "NAME_UNKNOWN", name)
		return
	}
	if upstream && s.UpstreamTags != nil {
		// Offline, only cached tags are listed.
		if remote, err := s.UpstreamTags(ctx, name); err == nil {
			tags = append(tags, remote...)
		} else if s.Logger != nil {
			s.Logger.Debugf("Cannot list tags of %s, using cached tags: %v", name, err)
		}
	}
	tags = unique(tags)
	if len(tags) == 0 {
		s.error(w, http.StatusNotFound, "NAME_UNKNOWN", name)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"name": repo, "tags": tags})
}

// resolve resolves a tag, pulling upstream images that are not cached.
func (s *Server) resolve(ctx context.Context, ref string, upstream bool) (descriptor, error) {
	desc, err := s.Store.Resolve(ctx, ref)
	if errors.Is(err, ErrNotFound) && upstream {
		if err := s.pull(ctx, ref); err != nil {
			return descriptor{}, err
		}
		desc, err = s.Store.Resolve(ctx, ref)
	}
	return descriptor{Digest: desc.Digest, MediaType: desc.MediaType}, err
}

// read returns content, pulling the upstream image ref when it is missing.
func (s *Server) read(ctx context.Context, dgst digest.Digest, ref string, upstream bool) ([]byte, error) {
	rc, err := s.Store.Open(ctx, dgst)
	if errors.Is(err, ErrNotFound) && upstream {
		if err := s.pull(ctx, ref); err != nil {
			return nil, err
		}
		rc, err = s.Store.Open(ctx, dgst)
	}
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxManifestSize))
}

func (s *Server) pull(ctx context.Context, ref string) error {
	if s.Logger != nil {
		s.Logger.Infof("Pulling %s into the image store", ref)
	}
	if err := s.Store.Pull(ctx, ref); err != nil {
		return fmt.Errorf("pull %s: %w", ref, err)
	}
	if err := s.Store.Cache(ctx, ref, s.LocalDomain+"/"+ref); err != nil {
		return fmt.Errorf("cache %s: %w", ref, err)
	}
	return nil
}

func (s *Server) storeError(w http.ResponseWriter, err error, code, what string) {
	if errors.Is(err, ErrNotFound) {
		s.error(w, http.StatusNotFound, code, what+" not found")
		return
	}
	if s.Logger != nil {
		s.Logger.Warnf("Image store error for %s: %v", what, err)
	}
	s.error(w, http.StatusBadGateway, "UNAVAILABLE", err.Error())
}

func (s *Server) error(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]string{{"code": code, "message": message}},
	})
}

type descriptor struct {
	Digest    digest.Digest
	MediaType string
}

// maxManifestSize bounds manifests read into memory.
const maxManifestSize = 4 << 20

// manifestMediaType returns the mediaType field of a manifest or index.
func manifestMediaType(data []byte) string {
	var m struct {
		MediaType string `json:"mediaType"`
	}
	if json.Unmarshal(data, &m) == nil && m.MediaType != "" {
		return m.MediaType
	}
	return "application/vnd.oci.image.manifest.v1+json"
}

// cutLast splits s around the last sep; callers check that s contains sep.
func cutLast(s, sep string) (string, string) {
	i := strings.LastIndex(s, sep)
	return s[:i], s[i+len(sep):]
}

func unique(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
