package environment

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
)

const (
	// minDockerFreeRatio is the minimum share of free space required on the
	// Docker data root. k3s evicts pods and taints the node with disk-pressure
	// below 5% free; the extra margin leaves room for pulling engine images.
	minDockerFreeRatio = 0.10

	dockerRootMountPath = "/docker"
)

// diskUsage holds filesystem size and free space in bytes.
type diskUsage struct {
	Total     uint64
	Available uint64
}

// checkDockerDiskSpace fails when the filesystem backing the Docker data root
// has less than minDockerFreeRatio free. The check runs `df` in a short-lived
// container so it measures the daemon's disk, which also works for Docker
// Desktop and remote DOCKER_HOST where the data root is not on this machine.
// If the check itself cannot run, it only logs a warning.
func checkDockerDiskSpace(ctx context.Context, dockerClient *docker.Client, image string, logger *zap.SugaredLogger) error {
	usage, err := dockerDiskUsage(ctx, dockerClient, image)
	if err != nil {
		logger.Warnf("Could not check free disk space on the Docker host: %v", err)
		return nil
	}
	return validateDiskUsage(usage)
}

// validateDiskUsage returns an error when free space is below minDockerFreeRatio.
func validateDiskUsage(usage diskUsage) error {
	if usage.Total == 0 {
		return nil
	}
	ratio := float64(usage.Available) / float64(usage.Total)
	if ratio >= minDockerFreeRatio {
		return nil
	}
	return fmt.Errorf("docker host is low on disk space: %s free of %s (%.0f%%), at least %.0f%% is required; free up space (e.g. docker system prune) and retry",
		formatBytes(usage.Available), formatBytes(usage.Total), ratio*100, minDockerFreeRatio*100)
}

// dockerDiskUsage runs `df` against the Docker data root in a throwaway
// container and returns the parsed usage.
func dockerDiskUsage(ctx context.Context, dockerClient *docker.Client, image string) (diskUsage, error) {
	info, err := dockerClient.Info(ctx)
	if err != nil {
		return diskUsage{}, fmt.Errorf("failed to get Docker info: %w", err)
	}
	if info.DockerRootDir == "" {
		return diskUsage{}, fmt.Errorf("docker did not report its data root")
	}

	resp, err := dockerClient.ContainerCreate(ctx,
		&container.Config{
			Image:      image,
			Entrypoint: []string{"df"},
			Cmd:        []string{"-Pk", dockerRootMountPath},
		},
		&container.HostConfig{
			Binds: []string{info.DockerRootDir + ":" + dockerRootMountPath + ":ro"},
		},
		nil, nil, "")
	if err != nil {
		return diskUsage{}, fmt.Errorf("failed to create disk check container: %w", err)
	}
	defer func() {
		_ = dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
	}()

	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return diskUsage{}, fmt.Errorf("failed to start disk check container: %w", err)
	}

	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return diskUsage{}, fmt.Errorf("failed waiting for disk check container: %w", err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return diskUsage{}, fmt.Errorf("disk check exited with code %d", status.StatusCode)
		}
	}

	logs, err := dockerClient.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return diskUsage{}, fmt.Errorf("failed to read disk check output: %w", err)
	}
	defer logs.Close()

	var stdout bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &bytes.Buffer{}, logs); err != nil {
		return diskUsage{}, fmt.Errorf("failed to read disk check output: %w", err)
	}
	return parseDfOutput(stdout.String())
}

// parseDfOutput parses POSIX `df -Pk` output (header plus one data line) and
// returns the total and available space in bytes.
func parseDfOutput(out string) (diskUsage, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return diskUsage{}, fmt.Errorf("unexpected df output: %q", out)
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 6 {
		return diskUsage{}, fmt.Errorf("unexpected df output: %q", out)
	}
	total, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return diskUsage{}, fmt.Errorf("invalid df size %q: %w", fields[1], err)
	}
	available, err := strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return diskUsage{}, fmt.Errorf("invalid df available %q: %w", fields[3], err)
	}
	return diskUsage{Total: total * 1024, Available: available * 1024}, nil
}

// formatBytes renders a byte count in GiB with one decimal place.
func formatBytes(b uint64) string {
	return fmt.Sprintf("%.1fGi", float64(b)/(1<<30))
}
