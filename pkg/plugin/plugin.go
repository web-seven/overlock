package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"github.com/alecthomas/kong"
)

const PluginPath = "./plugins"

func LoadPlugins() ([]kong.Option, error) {
	files, err := os.ReadDir(PluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	var options []kong.Option
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".so" {
			pluginPath := filepath.Join(PluginPath, file.Name())
			plug, err := plugin.Open(pluginPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load plugin: %v", err)
			}

			sym, err := plug.Lookup("RegisterCommands")
			if err != nil {
				return nil, fmt.Errorf("failed to find RegisterCommands function: %v", err)
			}

			registerPlugin, ok := sym.(func() []kong.Option)
			if !ok {
				return nil, fmt.Errorf("invalid plugin function signature: expected func() []kong.Option")
			}

			options = append(options, registerPlugin()...)
		}
	}
	return options, nil
}
