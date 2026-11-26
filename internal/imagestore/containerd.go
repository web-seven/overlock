package imagestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	contentapi "github.com/containerd/containerd/api/services/content/v1"
	imagesapi "github.com/containerd/containerd/api/services/images/v1"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	// criNamespace is the containerd namespace of the Kubernetes CRI.
	criNamespace = "k8s.io"
	// namespaceHeader selects the containerd namespace of a request.
	namespaceHeader = "containerd-namespace"
)

// ContainerdStore is a Store backed by a node's containerd, using only its
// image and content services and the CRI image service. It reads images and
// content, pulls images through the CRI, so node registry mirrors apply and
// pulled images are managed like images pulled by the kubelet, and labels and
// names pulled images. It never writes content, creates containers or deletes
// images or content.
type ContainerdStore struct {
	conn    *grpc.ClientConn
	images  imagesapi.ImagesClient
	content contentapi.ContentClient
	cri     runtimeapi.ImageServiceClient
}

// NewContainerdStore connects to the containerd socket.
func NewContainerdStore(socket string) (*ContainerdStore, error) {
	conn, err := grpc.Dial("unix://"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd at %s: %w", socket, err)
	}
	return &ContainerdStore{
		conn:    conn,
		images:  imagesapi.NewImagesClient(conn),
		content: contentapi.NewContentClient(conn),
		cri:     runtimeapi.NewImageServiceClient(conn),
	}, nil
}

// Close closes the connection.
func (s *ContainerdStore) Close() error {
	return s.conn.Close()
}

func withNamespace(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, namespaceHeader, criNamespace)
}

func notFound(err error) error {
	if status.Code(err) == codes.NotFound {
		return ErrNotFound
	}
	return err
}

// Resolve implements Store.
func (s *ContainerdStore) Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	resp, err := s.images.Get(withNamespace(ctx), &imagesapi.GetImageRequest{Name: ref})
	if err != nil {
		return ocispec.Descriptor{}, notFound(err)
	}
	t := resp.Image.Target
	return ocispec.Descriptor{MediaType: t.MediaType, Digest: digest.Digest(t.Digest), Size: t.Size}, nil
}

// Size implements Store.
func (s *ContainerdStore) Size(ctx context.Context, dgst digest.Digest) (int64, error) {
	resp, err := s.content.Info(withNamespace(ctx), &contentapi.InfoRequest{Digest: dgst.String()})
	if err != nil {
		return 0, notFound(err)
	}
	return resp.Info.Size, nil
}

// Open implements Store.
func (s *ContainerdStore) Open(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error) {
	if _, err := s.Size(ctx, dgst); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(withNamespace(ctx))
	stream, err := s.content.Read(ctx, &contentapi.ReadContentRequest{Digest: dgst.String()})
	if err != nil {
		cancel()
		return nil, notFound(err)
	}
	return &contentReader{stream: stream, cancel: cancel}, nil
}

// contentReader reads a content stream of containerd.
type contentReader struct {
	stream contentapi.Content_ReadClient
	cancel context.CancelFunc
	buf    []byte
}

func (r *contentReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 {
		resp, err := r.stream.Recv()
		if errors.Is(err, io.EOF) {
			return 0, io.EOF
		}
		if err != nil {
			return 0, notFound(err)
		}
		r.buf = resp.Data
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func (r *contentReader) Close() error {
	r.cancel()
	return nil
}

// Tags implements Store.
func (s *ContainerdStore) Tags(ctx context.Context, name string) ([]string, error) {
	resp, err := s.images.List(withNamespace(ctx), &imagesapi.ListImagesRequest{})
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, img := range resp.Images {
		if tag, ok := strings.CutPrefix(img.Name, name+":"); ok && !strings.ContainsAny(tag, "/@") {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// Pull implements Store.
func (s *ContainerdStore) Pull(ctx context.Context, ref string) error {
	_, err := s.cri.PullImage(ctx, &runtimeapi.PullImageRequest{Image: &runtimeapi.ImageSpec{Image: ref}})
	return err
}

// Labels of images kept by the store, as for images imported by K3s: managed
// by the CRI, and pinned against the kubelet's image garbage collection.
var cacheLabels = map[string]string{
	"io.cri-containerd.image":  "managed",
	"io.cri-containerd.pinned": "pinned",
}

// Cache implements Store.
func (s *ContainerdStore) Cache(ctx context.Context, ref, alias string) error {
	ctx = withNamespace(ctx)
	resp, err := s.images.Get(ctx, &imagesapi.GetImageRequest{Name: ref})
	if err != nil {
		return notFound(err)
	}
	for _, name := range []string{ref, alias} {
		img := &imagesapi.Image{Name: name, Labels: mergeLabels(resp.Image.Labels), Target: resp.Image.Target}
		if name == ref {
			_, err = s.images.Update(ctx, &imagesapi.UpdateImageRequest{Image: img})
		} else {
			_, err = s.images.Create(ctx, &imagesapi.CreateImageRequest{Image: img})
			if status.Code(err) == codes.AlreadyExists {
				_, err = s.images.Update(ctx, &imagesapi.UpdateImageRequest{Image: img})
			}
		}
		if err != nil {
			return fmt.Errorf("failed to keep image %s: %w", name, err)
		}
	}
	return nil
}

func mergeLabels(labels map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range labels {
		out[k] = v
	}
	for k, v := range cacheLabels {
		out[k] = v
	}
	return out
}
