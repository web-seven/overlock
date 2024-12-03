package configuration

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	cmv1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/fsnotify/fsnotify"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

func Watch(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger, path string) error {
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
				if event.Has(fsnotify.Write) {

					logger.Debugf("Changed file: %s", event)
					loadServed(ctx, dc, config, logger, path)
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
	packFiles, err := os.ReadDir(path)
	if err != nil {
		logger.Error(err)
	}

	for _, e := range packFiles {
		res := &metav1.TypeMeta{}
		yamlFile, err := os.ReadFile(fmt.Sprintf("%s/%s", path, e.Name()))
		if err != nil {
			logger.Error(err)
		}
		err = yaml.Unmarshal(yamlFile, res)
		if err != nil {
			logger.Error(err)
		}

		if res.Kind == "Configuration" {
			ccfg := &cmv1.Configuration{}
			err = yaml.Unmarshal(yamlFile, ccfg)
			if err != nil {
				logger.Error(err)
			}
			cfgName := fmt.Sprintf("%s:0.0.0", ccfg.GetName())
			cfg := New(cfgName)

			logger.Debugf("Upgrade Configuration: %s", cfg)
			err := cfg.UpgradeConfiguration(ctx, config, dc)

			logger.Infof("Changes detected, apply configuration: %s", ccfg.GetName())
			if err != nil {
				logger.Error(err)
			} else {

				logger.Debugf("Loading Configuration: %s", cfg)
				err = cfg.LoadDirectory(ctx, config, logger, path)
				if err != nil {
					logger.Error(err)
				} else {

					logger.Debugf("Loading Configuration: %s", cfg)
					err = cfg.Apply(ctx, config, logger)
					if err != nil {
						logger.Error(err)
					}
				}
			}
		}
	}
}
