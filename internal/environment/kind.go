package environment

import (
	"bufio"
	"os/exec"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
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

func (e *Environment) CreateKindEnvironment(logger *zap.Logger) (string, error) {

	clusterYaml := e.configYaml(logger)

	cmd := exec.Command("kind", "create", "cluster", "--name", e.name, "--config", "-")
	cmd.Stdin = strings.NewReader(clusterYaml)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Sugar().Errorf("error creating StderrPipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Sugar().Errorf("error creating StdoutPipe: %v", err)
	}

	cmd.Start()

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		line := stderrScanner.Text()
		if strings.Contains(line, "ERROR") {
			logger.Sugar().Fatal(line)
		} else {
			if !strings.Contains(line, " • ") {
				logger.Sugar().Debug(line)
			}
		}
	}

	stdoutScanner := bufio.NewScanner(stdout)
	for stdoutScanner.Scan() {
		line := stdoutScanner.Text()
		if !strings.Contains(line, " • ") {
			logger.Sugar().Debug(line)
		}
	}

	cmd.Wait()
	return e.KindContextName(), nil
}

func (e *Environment) DeleteKindEnvironment(logger *zap.Logger) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", e.name)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Start()

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		logger.Sugar().Info(stderrScanner.Text())
	}
	return nil
}

func (e *Environment) KindContextName() string {
	return "kind-" + e.name
}

// Return YAML of cluster config file
func (e *Environment) configYaml(logger *zap.Logger) string {

	template := KindCluster{
		Kind:       "Cluster",
		APIVersion: "kind.x-k8s.io/v1alpha4",
		Nodes: []KindNode{
			{
				Role:        "worker",
				ExtraMounts: []KindMount{},
			},
			{
				Role: "control-plane",
				KubeadmConfigPatches: []string{
					`kind: InitConfiguration
nodeRegistration:
  kubeletExtraArgs:
    node-labels: "ingress-ready=true"`,
				},
				ExtraPortMappings: []KindPortMapping{
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
				},
			},
		},
	}

	if e.mountPath != "" {
		template.Nodes[0].ExtraMounts = append(template.Nodes[0].ExtraMounts, KindMount{
			HostPath:      e.mountPath,
			ContainerPath: "/storage",
		})
	}

	yamlData, err := yaml.Marshal(&template)
	if err != nil {
		logger.Sugar().Fatalf("error: %v", err)
	}
	return string(yamlData)
}
