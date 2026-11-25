// Command k3s-image prepares the build context of the Overlock K3s image:
//
//	k3s-image images --operator-image IMG     images to preload, one per line
//	k3s-image manifests --operator-image IMG  operator bootstrap manifests
//	k3s-image k3s-version                     default k3s version
//
// Images are collected by rendering the charts the operator installs with the
// same versions and values, so the bundle follows the operator.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	"github.com/web-seven/overlock/internal/bootstrap"
	"github.com/web-seven/overlock/internal/controller"
	"github.com/web-seven/overlock/pkg/environment"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "k3s-image:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: k3s-image images|manifests|k3s-version [--operator-image IMG]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	operatorImage := fs.String("operator-image", "", "operator image")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	switch args[0] {
	case "k3s-version":
		_, err := fmt.Fprintln(out, environment.K3sDefaultVersion)
		return err
	case "manifests":
		if *operatorImage == "" {
			return errors.New("--operator-image is required")
		}
		return writeManifests(out, *operatorImage)
	case "images":
		if *operatorImage == "" {
			return errors.New("--operator-image is required")
		}
		images, err := chartImages(controller.DefaultCharts())
		if err != nil {
			return err
		}
		images = append(images, normalize(*operatorImage))
		for _, img := range unique(images) {
			if _, err := fmt.Fprintln(out, img); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func writeManifests(out io.Writer, image string) error {
	objs, err := bootstrap.Manifests(bootstrap.Options{Image: image})
	if err != nil {
		return err
	}
	for _, obj := range objs {
		data, err := yaml.Marshal(obj.Object)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "---\n%s", data); err != nil {
			return err
		}
	}
	return nil
}

// chartImages renders each chart client-side and returns the container images
// of its workloads and hooks.
func chartImages(charts []controller.Chart) ([]string, error) {
	dir, err := os.MkdirTemp("", "k3s-image-charts")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	var images []string
	for _, c := range charts {
		manifest, err := render(c, dir)
		if err != nil {
			return nil, fmt.Errorf("failed to render %s: %w", c.Name, err)
		}
		found, err := manifestImages(manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to read images of %s: %w", c.Name, err)
		}
		if len(found) == 0 {
			return nil, fmt.Errorf("no images found in chart %s", c.Name)
		}
		images = append(images, found...)
	}
	return images, nil
}

func render(c controller.Chart, dir string) (string, error) {
	pull := action.NewPullWithOpts(action.WithConfig(&action.Configuration{}))
	pull.Settings = cli.New()
	pull.RepoURL = c.RepoURL
	pull.Version = c.Version
	pull.DestDir = dir
	if _, err := pull.Run(c.Chart); err != nil {
		return "", err
	}
	ch, err := loader.Load(filepath.Join(dir, fmt.Sprintf("%s-%s.tgz", c.Chart, c.Version)))
	if err != nil {
		return "", err
	}

	install := action.NewInstall(&action.Configuration{})
	install.DryRun = true
	install.ClientOnly = true
	install.Replace = true
	install.ReleaseName = c.Release
	install.Namespace = c.Namespace
	install.KubeVersion = &chartutil.KubeVersion{Version: "v1.36.0", Major: "1", Minor: "36"}
	rel, err := install.Run(ch, c.Values)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(rel.Manifest)
	for _, h := range rel.Hooks {
		// Only install hooks run when an environment is created; upgrade,
		// delete and test hooks pull their images when they run.
		if !isInstallHook(h) {
			continue
		}
		b.WriteString("\n---\n")
		b.WriteString(h.Manifest)
	}
	return b.String(), nil
}

func isInstallHook(h *release.Hook) bool {
	for _, e := range h.Events {
		if e == release.HookPreInstall || e == release.HookPostInstall {
			return true
		}
	}
	return false
}

// podSpecPaths maps workload kinds to the path of their pod spec.
var podSpecPaths = map[string][]string{
	"Pod":         {"spec"},
	"Deployment":  {"spec", "template", "spec"},
	"StatefulSet": {"spec", "template", "spec"},
	"DaemonSet":   {"spec", "template", "spec"},
	"ReplicaSet":  {"spec", "template", "spec"},
	"Job":         {"spec", "template", "spec"},
	"CronJob":     {"spec", "jobTemplate", "spec", "template", "spec"},
}

func manifestImages(manifest string) ([]string, error) {
	var images []string
	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(manifest)), 4096)
	for {
		obj := &unstructured.Unstructured{}
		if err := dec.Decode(&obj.Object); err != nil {
			if errors.Is(err, io.EOF) {
				return images, nil
			}
			return nil, err
		}
		path, ok := podSpecPaths[obj.GetKind()]
		if !ok {
			continue
		}
		for _, field := range []string{"initContainers", "containers"} {
			containers, _, _ := unstructured.NestedSlice(obj.Object, append(append([]string{}, path...), field)...)
			for _, c := range containers {
				if m, ok := c.(map[string]any); ok {
					if img, ok := m["image"].(string); ok && img != "" {
						images = append(images, normalize(img))
					}
				}
			}
		}
	}
}

// normalize returns the fully qualified reference containerd and the kubelet
// use, so preloaded images match the references in pod specs.
func normalize(img string) string {
	first := strings.SplitN(img, "/", 2)[0]
	if !strings.Contains(img, "/") || (!strings.ContainsAny(first, ".:") && first != "localhost") {
		if !strings.Contains(img, "/") {
			img = "library/" + img
		}
		img = "docker.io/" + img
	}
	if !strings.Contains(img[strings.LastIndex(img, "/")+1:], ":") && !strings.Contains(img, "@") {
		img += ":latest"
	}
	return img
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
