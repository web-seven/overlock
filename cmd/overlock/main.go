package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/web-seven/overlock/cmd/overlock/configuration"
	"github.com/web-seven/overlock/cmd/overlock/environment"
	"github.com/web-seven/overlock/cmd/overlock/function"
	"github.com/web-seven/overlock/cmd/overlock/provider"
	"github.com/web-seven/overlock/cmd/overlock/version"
	"github.com/web-seven/overlock/internal/engine"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/namespace"

	pluginPkg "github.com/web-seven/overlock/pkg/plugin"

	"github.com/willabides/kongplete"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/web-seven/overlock/cmd/overlock/registry"
	"github.com/web-seven/overlock/cmd/overlock/resource"
)

type Globals struct {
	Debug         bool        `short:"D" help:"Enable debug mode"`
	Version       VersionFlag `name:"version" help:"Print version information and quit"`
	Namespace     string      `name:"namespace" short:"n" help:"Namespace used for cluster resources"`
	EngineRelease string      `name:"engine-release" short:"r" help:"Crossplane Helm release name"`
	EngineVersion string      `name:"engine-version" default:"1.19.0" short:"v" help:"Crossplane version"`
	PluginPath    string      `name:"plugin-path" help:"Path to the plugin file" default:"${homedir}/.config/overlock/plugins"`
}

type VersionFlag string

func (v VersionFlag) Decode(ctx *kong.DecodeContext) error { return nil }
func (v VersionFlag) IsBool() bool                         { return true }
func (v VersionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Println(vars["version"])
	app.Exit(0)
	return nil
}

func getDescriptionText() string {
	return ""
}

// customHelpPrinter prints the banner before showing the standard Kong help
func customHelpPrinter(options kong.HelpOptions, ctx *kong.Context) error {
	// Print banner first
	fmt.Println(displayBanner())
	// Then print standard help
	return kong.DefaultHelpPrinter(options, ctx)
}

func displayBanner() string {
	// Define styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")). // Cyan
		Bold(true)

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")). // Light cyan for main text
		Align(lipgloss.Center)

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")). // White
		Align(lipgloss.Center)

	urlStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")). // Gray
		Align(lipgloss.Center)

	// Create content with proper alignment
	content := fmt.Sprintf("%s\n\n%s\n%s",
		contentStyle.Render("Crossplane Environment Management"),
		versionStyle.Render(fmt.Sprintf("Version: %s", version.Version)),
		urlStyle.Render("https://github.com/overlock-network/overlock"),
	)

	// Create the box with fixed width
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")). // Cyan
		Padding(1, 4).
		Width(60).
		Align(lipgloss.Center)

	// Create title
	title := titleStyle.Render("Overlock")
	titleBox := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(52). // Width minus padding and border
		Render(title)

	// Combine title and content
	bannerContent := boxStyle.Render(titleBox + "\n" + content)

	return bannerContent + "\n"
}

func (c *cli) AfterApply(ctx *kong.Context) error { //nolint:unparam
	config, err := ctrl.GetConfig()
	if err != nil {
		// Config is optional - may not be available in some contexts
		config = nil
	}
	if config != nil {
		ctx.Bind(config)
		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create dynamic client: %w", err)
		}
		kubeClient, err := kube.Client(config)
		if err != nil {
			return fmt.Errorf("failed to create kube client: %w", err)
		}
		ctx.Bind(dynamicClient)
		ctx.Bind(kubeClient)
	}

	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	cfg.EncoderConfig.TimeKey = ""
	cfg.EncoderConfig.CallerKey = ""
	if c.Globals.Debug {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}

	if os.Getenv(namespace.OVERLOCK_ENGINE_NAMESPACE) != "" {
		namespace.Namespace = os.Getenv(namespace.OVERLOCK_ENGINE_NAMESPACE)
	} else if c.Globals.Namespace != "" {
		namespace.Namespace = c.Globals.Namespace
	}

	if os.Getenv(engine.OVERLOCK_ENGINE_RELEASE) != "" {
		engine.AltRelease = os.Getenv(engine.OVERLOCK_ENGINE_RELEASE)
	} else if c.Globals.EngineRelease != "" {
		engine.AltRelease = c.Globals.EngineRelease
	}

	if os.Getenv(engine.OVERLOCK_ENGINE_VERSION) != "" {
		engine.Version = os.Getenv(engine.OVERLOCK_ENGINE_VERSION)
	} else if c.Globals.EngineVersion != "" {
		engine.Version = c.Globals.EngineVersion
	}

	logger, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}
	ctrl.SetLogger(logr.Logger{})
	ctx.Bind(logger.Sugar())
	return nil
}

type cli struct {
	Globals

	Help               helpCmd                      `cmd:"" help:"Show help."`
	Environment        environment.Cmd              `cmd:"" name:"environment" aliases:"env" help:"Overlock Environment commands"`
	Configuration      configuration.Cmd            `cmd:"" name:"configuration" aliases:"cfg" help:"Overlock Configuration commands"`
	Resource           resource.Cmd                 `cmd:"" name:"resource" aliases:"res" help:"Overlock Resource commands"`
	Registry           registry.Cmd                 `cmd:"" name:"registry" aliases:"reg" help:"Packages registry commands"`
	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`
	Provider           provider.Cmd                 `cmd:"" name:"provider" aliases:"prv" help:"Overlock Provider commands"`
	Function           function.Cmd                 `cmd:"" name:"function" aliases:"fnc" help:"Overlock Function commands"`
	// Search             registry.SearchCmd           `cmd:"" help:"Search for packages"`
	// Generate           generate.Cmd                 `cmd:"" help:"Generate example by XRD YAML file"`
}

type helpCmd struct{}

func main() {
	homeDir, _ := os.UserHomeDir()
	c := cli{
		Globals: Globals{
			Version:    VersionFlag(version.Version),
			PluginPath: pluginPkg.PluginPath,
		},
	}
	pluginOptions, err := pluginPkg.LoadPlugins()
	if err != nil {
		fmt.Println("Warning:", err)
		pluginOptions = []kong.Option{}
	}

	parser := kong.Must(&c,
		append([]kong.Option{
			kong.Name("overlock"),
			kong.Description(getDescriptionText()),
			kong.Help(customHelpPrinter),
			kong.Vars{
				"version": version.Version,
				"homedir": homeDir,
			},
			kong.ConfigureHelp(kong.HelpOptions{
				NoExpandSubcommands: true,
				Compact:             true,
			}),
		}, pluginOptions...)...)

	kongplete.Complete(parser)

	if len(os.Args) == 1 {
		_, err := parser.Parse([]string{"--help"})
		parser.FatalIfErrorf(err)
		return
	}

	kongCtx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		defer cancel()
		<-sigCh
		kongCtx.Exit(1)
	}()

	kongCtx.BindTo(ctx, (*context.Context)(nil))
	kongCtx.FatalIfErrorf(kongCtx.Run())
}
