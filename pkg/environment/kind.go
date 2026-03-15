package environment

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v3"

	overlockerrors "github.com/web-seven/overlock/pkg/errors"
)

type KindCluster struct {
	Kind       string     `yaml:"kind"`
	APIVersion string     `yaml:"apiVersion"`
	Nodes      []KindNode `yaml:"nodes"`
}

type KindNode struct {
	Role                 string            `yaml:"role"`
	ExtraMounts          []KindMount       `yaml:"extraMounts,omitempty"`
	KubeadmConfigPatches []string          `yaml:"kubeadmConfigPatches,omitempty"`
	ExtraPortMappings    []KindPortMapping `yaml:"extraPortMappings,omitempty"`
}

type KindMount struct {
	HostPath      string `yaml:"hostPath"`
	ContainerPath string `yaml:"containerPath"`
}

type KindPortMapping struct {
	ContainerPort int    `yaml:"containerPort"`
	HostPort      int    `yaml:"hostPort"`
	Protocol      string `yaml:"protocol"`
}

func (e *Environment) CreateKindEnvironment(logger *zap.SugaredLogger) (string, error) {
	// Check if cluster already exists
	checkCmd := exec.Command("kind", "get", "clusters")
	output, err := checkCmd.Output()
	if err == nil {
		clusters := strings.Split(string(output), "\n")
		for _, cluster := range clusters {
			if strings.TrimSpace(cluster) == e.name {
				logger.Infof("Environment '%s' already exists. Using existing environment.", e.name)
				return e.KindContextName(), nil
			}
		}
	}

	var cmd *exec.Cmd

	if e.engineConfig != "" {
		cmd = exec.Command("kind", "create", "cluster", "--name", e.name, "--config", e.engineConfig)
	} else {
		clusterYaml, err := e.configYaml(logger)
		if err != nil {
			return "", overlockerrors.NewInvalidConfigErrorWithCause("", "", "failed to generate cluster config", err)
		}
		cmd = exec.Command("kind", "create", "cluster", "--name", e.name, "--config", "-")
		cmd.Stdin = strings.NewReader(clusterYaml)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Errorf("error creating StderrPipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Errorf("error creating StdoutPipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start kind command: %w", err)
	}

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		line := stderrScanner.Text()
		if strings.Contains(line, "ERROR") {
			return "", errors.Wrap(errors.New(line), "kind cluster creation failed")
		}
		if !strings.Contains(line, " • ") {
			logger.Debug(line)
		}
	}

	stdoutScanner := bufio.NewScanner(stdout)
	for stdoutScanner.Scan() {
		line := stdoutScanner.Text()
		if !strings.Contains(line, " • ") {
			logger.Debug(line)
		}
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("kind command failed: %w", err)
	}
	return e.KindContextName(), nil
}

func (e *Environment) DeleteKindEnvironment(logger *zap.SugaredLogger) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", e.name)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start kind delete command: %w", err)
	}

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		logger.Info(stderrScanner.Text())
	}
	return nil
}

func (e *Environment) KindContextName() string {
	return "kind-" + e.name
}

// Return YAML of cluster config file
func (e *Environment) configYaml(_ *zap.SugaredLogger) (string, error) {
	ports := []KindPortMapping{
		{
			ContainerPort: 80,
			HostPort:      e.httpPort,
			Protocol:      "TCP",
		},
		{
			ContainerPort: 443,
			HostPort:      e.httpsPort,
			Protocol:      "TCP",
		},
	}
	if e.disablePorts {
		ports = []KindPortMapping{}
	}
	template := KindCluster{
		Kind:       "Cluster",
		APIVersion: "kind.x-k8s.io/v1alpha4",
		Nodes: []KindNode{
			{
				Role: "control-plane",
				KubeadmConfigPatches: []string{
					`kind: InitConfiguration
nodeRegistration:
  kubeletExtraArgs:
    node-labels: "ingress-ready=true"`,
				},

				ExtraPortMappings: ports,
			},
		},
	}

	if e.mountPath != "" {
		template.Nodes[0].ExtraMounts = append(template.Nodes[0].ExtraMounts, KindMount{
			HostPath:      e.mountPath,
			ContainerPath: e.containerPath,
		})
	}

	yamlData, err := yaml.Marshal(&template)
	if err != nil {
		return "", overlockerrors.NewInvalidConfigErrorWithCause("", "", "failed to marshal cluster configuration template", err)
	}
	return string(yamlData), nil
}
