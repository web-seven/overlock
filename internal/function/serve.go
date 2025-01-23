package function

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cmv1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	"github.com/rjeczalik/notify"
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
					loadServed(ctx, dc, config, logger, path)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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
			if filepath.Ext(e.Name()) != ".yaml" {
				continue
			}

			res := &metav1.TypeMeta{}
			yamlFile, err := os.ReadFile(fmt.Sprintf("%s/%s", packagePath, e.Name()))
			if err != nil {
				logger.Error(err)
			}
			err = yaml.Unmarshal(yamlFile, res)
			if err != nil {
				logger.Error(err)
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
