// Command imagestore serves the images of a node's containerd through the
// read-only OCI distribution API, with pull-through caching of upstream
// registries. It runs on every node as part of an Overlock environment, so
// Crossplane installs packages without a separate registry.
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.uber.org/zap"

	"github.com/web-seven/overlock/internal/imagestore"
)

func main() {
	socket := flag.String("containerd-socket", "/run/k3s/containerd/containerd.sock", "Path of the containerd socket.")
	listen := flag.String("listen", ":8443", "Address to serve the registry API on.")
	tlsCert := flag.String("tls-cert", "/tls/tls.crt", "TLS certificate file.")
	tlsKey := flag.String("tls-key", "/tls/tls.key", "TLS key file.")
	localDomain := flag.String("local-domain", "registry.overlock.svc.cluster.local", "Registry name of images without upstream registry.")
	flag.Parse()

	zl, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	logger := zl.Sugar()

	store, err := imagestore.NewContainerdStore(*socket)
	if err != nil {
		logger.Fatal(err)
	}
	defer store.Close()

	server := &http.Server{
		Addr: *listen,
		Handler: &imagestore.Server{
			Store:        store,
			LocalDomain:  *localDomain,
			UpstreamTags: upstreamTags,
			Logger:       logger,
		},
		ReadHeaderTimeout: 30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	logger.Infof("Serving containerd images on %s", *listen)
	if err := server.ListenAndServeTLS(*tlsCert, *tlsKey); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal(err)
	}
}

// upstreamTags lists the tags of a public upstream repository.
func upstreamTags(ctx context.Context, repo string) ([]string, error) {
	r, err := name.NewRepository(repo)
	if err != nil {
		return nil, err
	}
	return remote.List(r, remote.WithContext(ctx))
}
