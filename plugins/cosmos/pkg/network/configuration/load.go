package configuration

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/google/go-containerregistry/pkg/crane"
	conregv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type contextKey string

const authContextKey contextKey = "auth"

const maxDecompressedSize = 200 * 1024 * 1024

func fetchPackage(ctx context.Context, refName string, key string) ([]map[string]interface{}, []string) {

	var yamlSchemas []map[string]interface{}
	var errors []string
	var extractedSchemas [][]byte

	layer, err := FetchBaseLayer(ctx, refName)
	if err != nil {
		if err.Error() == "cannot get image labels" {
			layers, err := FetchImage(ctx, refName)
			if err != nil {
				log.Printf("Failed to fetch image %s: %v", refName, err)
				errors = append(errors, "Failed to fetch image "+refName)
				return nil, errors
			}

			extractedSchemas, err = ExtractPackageCRDs(layers)
			if err != nil {
				log.Printf("Failed to extract CRDs from image %s: %v", refName, err)
				errors = append(errors, "Failed to extract CRDs from image "+refName)
				return nil, errors
			}
		} else {
			log.Printf("Failed to download package %s: %v", refName, err)
			errors = append(errors, "Failed to download image "+refName)
			return nil, errors
		}
	} else {
		extractedSchemas, err = ExtractPackageContent(*layer)

		if err != nil {
			log.Printf("Error extracting base layer for %s: %v", refName, err)
			errors = append(errors, "Failed to extract base layer for "+refName)
			return nil, errors
		}
	}

	var controlCharRegex = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)

	for _, schema := range extractedSchemas {

		cleanSchema := controlCharRegex.ReplaceAll(schema, []byte(""))
		aesKey, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			log.Printf("Failed to decode AES key: %v", err)
		}

		decrypted, err := decryptPackage(string(cleanSchema), aesKey)
		if err == nil {
			cleanSchema = decrypted

		} else {
			log.Printf("Decryption failed for %s: %v", refName, err)
		}

		decoder := yaml.NewYAMLOrJSONDecoder(io.NopCloser(bytes.NewReader(cleanSchema)), 4096)

		for {
			var yamlSchema map[string]interface{}
			if err := decoder.Decode(&yamlSchema); err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("Skipping invalid YAML schema in %s: %v", refName, err)
				errors = append(errors, "Skipped invalid schema in "+refName)
				break
			}

			if yamlSchema == nil {
				continue
			}

			yamlSchemas = append(yamlSchemas, yamlSchema)
		}
	}

	return yamlSchemas, errors

}

func FetchBaseLayer(ctx context.Context, image string) (*conregv1.Layer, error) {
	const baseLayerLabel = "base"
	const refFmt = "%s@%s"

	image, err := prepareImageReference(image)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare image reference")
	}
	var cBytes []byte
	cBytes, err = crane.Config(image)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get config")
	}
	cfg := &conregv1.ConfigFile{}
	if err := yaml.Unmarshal(cBytes, cfg); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal image config")
	}
	if cfg.Config.Labels == nil {
		return nil, errors.New("cannot get image labels")
	}
	var label string

	ls := cfg.Config.Labels
	for v, k := range ls {
		if k == baseLayerLabel {
			label = v // e.g.: io.crossplane.xpkg:sha256:0158764f65dc2a68728fdffa6ee6f2c9ef158f2dfed35abbd4f5bef8973e4b59
		}
	}
	if label == "" {
		fmt.Println("No base layer found in image labels")
	}
	lDigest := strings.SplitN(label, ":", 2)[1]

	ll, err := crane.PullLayer(fmt.Sprintf(refFmt, image, lDigest))
	if err != nil {
		return nil, errors.Wrapf(err, "cannot pull base layer %s", lDigest)
	}
	return &ll, nil
}

func prepareImageReference(image string) (string, error) {
	if strings.Contains(image, "@") {
		return strings.SplitN(image, "@", 2)[0], nil
	}
	if strings.Contains(image, ":") {
		return findImageTagForVersionConstraint(image)
	}
	return image, nil
}

func findImageTagForVersionConstraint(image string) (string, error) {
	// Separate the image base and the image tag
	parts := strings.Split(image, ":")
	lastPart := len(parts) - 1
	imageBase := strings.Join(parts[0:lastPart], ":")
	imageTag := parts[lastPart]

	// Check if the tag is a constraint
	isConstraint := true
	c, err := semver.NewConstraint(imageTag)
	if err != nil {
		isConstraint = false
	}

	// Return original image if no constraint was detected
	if !isConstraint {
		return image, nil
	}

	// Fetch all image tags
	var tags []string
	tags, err = crane.ListTags(imageBase)
	if err != nil {
		return "", errors.Wrapf(err, "cannot fetch tags for the image %s", imageBase)
	}

	// Convert tags to semver versions
	vs := []*semver.Version{}
	for _, r := range tags {
		v, err := semver.NewVersion(r)
		if err != nil {
			// We skip any tags that are not valid semantic versions
			continue
		}
		vs = append(vs, v)
	}

	// Sort all versions and find the last version complient with the constraint
	sort.Sort(sort.Reverse(semver.Collection(vs)))
	var addVer string
	for _, v := range vs {
		if c.Check(v) {
			addVer = v.Original()

			break
		}
	}

	if addVer == "" {
		return "", errors.Errorf("cannot find any tag complient with the constraint %s", imageTag)
	}

	// Compose new complete image string if any complient version was found
	image = fmt.Sprintf("%s:%s", imageBase, addVer)

	return image, nil
}

func FetchImage(ctx context.Context, image string) ([]conregv1.Layer, error) {
	image, err := prepareImageReference(image)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare image reference")
	}
	var img conregv1.Image
	img, err = crane.Pull(image)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to pull image")
	}
	layers, err := img.Layers()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get image layers")
	}
	return layers, nil
}

func ExtractPackageCRDs(layers []conregv1.Layer) ([][]byte, error) {
	tmpDir, err := os.MkdirTemp("", "image-extract")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create temporary directory")
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("Failed to remove temporary directory: %v", err)
		}
	}()

	for _, layer := range layers {
		if err := extractLayer(layer, tmpDir); err != nil {
			return nil, errors.Wrapf(err, "failed to extract layer")
		}
	}

	var yamlFiles [][]byte
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "package.yaml" {
			content, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				return errors.Wrapf(err, "failed to read file: %s", path)
			}
			yamlFiles = append(yamlFiles, content)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to walk through extracted files")
	}
	return yamlFiles, nil
}

func ExtractPackageContent(layer conregv1.Layer) ([][]byte, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get uncompressed layer")
	}
	defer rc.Close()

	objs, err := load(rc)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read from layer")
	}

	if len(objs) > 0 {
		firstObj := objs[0]
		firstObjLines := strings.SplitN(string(firstObj), "\n", 2)
		if len(firstObjLines) > 1 {
			objs[0] = []byte(firstObjLines[1])
		} else {
			objs[0] = []byte{}
		}
	}
	return objs, nil
}

func load(r io.Reader) ([][]byte, error) {
	stream := make([][]byte, 0)

	yr := yaml.NewYAMLReader(bufio.NewReader(r))

	for {
		bytes, err := yr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "cannot parse YAML stream")
		}
		if len(bytes) == 0 {
			continue
		}
		stream = append(stream, bytes)
	}

	return stream, nil
}

func extractLayer(layer conregv1.Layer, destDir string) error {
	r, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Printf("Failed to close reader: %v", err)
		}
	}()

	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break // End of tar archive
		}
		if err != nil {
			return err
		}

		// Resolve the target path
		target := filepath.Join(destDir, filepath.Clean(hdr.Name))
		targetPath, err := filepath.Abs(target)
		if err != nil {
			return errors.Wrap(err, "failed to get absolute path")
		}

		// Skip entries that are the same as the destination directory or just "./"
		if targetPath == filepath.Clean(destDir) || hdr.Name == "./" {
			continue
		}

		// Ensure the target path is within the destination directory
		if !strings.HasPrefix(targetPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return errors.Errorf("invalid file path: %s", targetPath)
		}

		// Create the file or directory
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o750); err != nil {
				return errors.Wrapf(err, "cannot create directory: %s", targetPath)
			}
		case tar.TypeReg:
			dir := filepath.Dir(targetPath)
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return errors.Wrapf(err, "cannot create directory: %s", dir)
			}
			file, err := os.Create(filepath.Clean(targetPath))
			if err != nil {
				return errors.Wrapf(err, "cannot create file: %s", targetPath)
			}
			defer func() {
				if err := file.Close(); err != nil {
					log.Printf("Failed to close file: %v", err)
				}
			}()

			// Limit the decompression size to avoid DoS attacks
			limitedReader := io.LimitReader(tr, maxDecompressedSize)
			if _, err := io.Copy(file, limitedReader); err != nil {
				return errors.Wrapf(err, "cannot decompress file: %s", targetPath)
			}
		}
	}

	return nil
}
func LoadBinaryLayerStream(content []byte, fileName string, permissions fs.FileMode) (conregv1.Layer, error) {

	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	err := tw.WriteHeader(&tar.Header{
		Name: fileName,
		Mode: int64(permissions),
		Size: int64(len(content)),
	})
	if err != nil {
		return nil, err
	}

	_, err = tw.Write(content)
	if err != nil {
		return nil, err
	}

	err = tw.Close()
	if err != nil {
		return nil, err
	}

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	})
	if err != nil {
		return nil, err
	}

	return layer, nil
}
