package environment

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/pterm/pterm"
	"k8s.io/client-go/tools/clientcmd"
)

type listCmd struct {
}

func (c *listCmd) Run(ctx context.Context, logger *log.Logger) error {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	config := clientcmd.GetConfigFromFileOrDie(kubeconfig)

	tableData := pterm.TableData{{"NAME", "TYPE"}}
	for name := range config.Contexts {
		types := regexp.MustCompile(`(\w+)`).FindStringSubmatch(name)
		tableData = append(tableData, []string{name, strings.ToUpper(types[0])})
	}

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	return nil
}
