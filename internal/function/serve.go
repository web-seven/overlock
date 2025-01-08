package function

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cmv1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	"github.com/fsnotify/fsnotify"
	"github.com/web-seven/overlock/internal/packages"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

func Serve(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger, path string) error {
	logger.Infof("Started serve path: %s", path)

	loadServed(ctx, dc, config, logger, path)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) ||
					event.Has(fsnotify.Create) ||
					event.Has(fsnotify.Remove) ||
					event.Has(fsnotify.Rename) {
					fileExt := filepath.Ext(event.Name)
					if fileExt == ".yaml" || fileExt == ".go" {
						logger.Debugf("Changed file: %s", event)
						loadServed(ctx, dc, config, logger, path)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error(err)
			}
		}
	}()

	err = watcher.Add(path)
	if err != nil {
		logger.Error(err)
	}

	<-make(chan struct{})
	return nil
}

func loadServed(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger, path string) {
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
			if res.Kind == "Function" {
				cfnc := &cmv1beta1.Function{}
				err = yaml.Unmarshal(yamlFile, cfnc)
				if err != nil {
					logger.Error(err)
				}
				fncName := fmt.Sprintf("%s:0.0.0", cfnc.GetName())
				fnc := New(fncName)

				logger.Debugf("Upgrade function: %s", fnc)
				err := fnc.UpgradeFunction(ctx, config, dc)

				logger.Infof("Changes detected, apply function: %s", cfnc.GetName())
				if err != nil {
					logger.Error(err)
				} else {

					logger.Debugf("Loading function: %s", fnc)
					err = fnc.LoadDirectory(ctx, config, logger, path)
					if err != nil {
						logger.Error(err)
					} else {

						logger.Debugf("Loading function: %s", fnc)
						err = fnc.Apply(ctx, config, logger)
						if err != nil {
							logger.Error(err)
						}
					}
				}
			}
		}
	}
}
