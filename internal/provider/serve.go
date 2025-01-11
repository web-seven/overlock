package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cmv1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/rjeczalik/notify"
	"github.com/web-seven/overlock/internal/packages"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

func Serve(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger, path string, mainPath string) error {
	logger.Infof("Started serve path: %s", path)

	loadServed(ctx, dc, config, logger, path, mainPath)

	c := make(chan notify.EventInfo, 1)

	if err := notify.Watch(fmt.Sprintf("%s/%s", path, "..."), c, notify.Create, notify.Write, notify.Rename, notify.Remove); err != nil {
		return err
	}
	defer notify.Stop(c)

	go func() {
		for {
			select {
			case ev := <-c:
				fileExt := filepath.Ext(ev.Path())
				if fileExt == ".yaml" || fileExt == ".go" {
					logger.Debugf("Changed file: %s", ev)
					loadServed(ctx, dc, config, logger, path, mainPath)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	<-make(chan struct{})
	return nil
}

// Build and load served provider into k8s context
func loadServed(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger, path string, mainPath string) {
	packagePath := fmt.Sprintf("%s/%s", path, packages.PackagePath)
	packFiles, err := os.ReadDir(packagePath)
	if err != nil {
		logger.Error(err)
	}

	for _, e := range packFiles {
		if e.Type().IsRegular() {
			res := &metav1.TypeMeta{}
			yamlFile, err := os.ReadFile(fmt.Sprintf("%s/%s", packagePath, e.Name()))
			if err != nil {
				logger.Error(err)
			}
			err = yaml.Unmarshal(yamlFile, res)
			if err != nil {
				logger.Infof("Found non YAML file in package directory: %s. Please move it out before build package.", e.Name())
				continue
			}
			logger.Debugf("Package found with kind: %s", res.Kind)
			if res.Kind == "Provider" {
				cpvd := &cmv1.Provider{}
				err = yaml.Unmarshal(yamlFile, cpvd)
				if err != nil {
					logger.Error(err)
				}
				pvdName := fmt.Sprintf("%s:0.0.0", cpvd.GetName())
				pvd := New(pvdName)

				logger.Debugf("Upgrade provider: %s", pvd)
				err := pvd.UpgradeProvider(ctx, config, dc, logger)

				logger.Infof("Changes detected, apply provider: %s", cpvd.GetName())
				if err != nil {
					logger.Error(err)
				} else {

					logger.Debugf("Loading provider: %s", pvd)
					err = pvd.LoadDirectory(ctx, config, logger, path, mainPath)
					if err != nil {
						logger.Error(err)
					} else {

						logger.Debugf("Loading provider: %s", pvd)
						err = pvd.ApplyPackage(ctx, config, logger)
						if err != nil {
							logger.Error(err)
						}
					}
				}
			}
		}
	}
}
