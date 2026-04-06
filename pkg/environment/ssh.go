package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSHClient wraps an SSH connection to a remote host for executing Docker
// commands remotely.
type SSHClient struct {
	Host   string
	User   string
	Port   int
	Key    string // path to private key
	client *ssh.Client
}

// NewSSHClient creates and connects an SSH client to the remote host.
// If keyPath is empty, ~/.ssh/id_rsa is used.
func NewSSHClient(host, user string, port int, keyPath string) (*SSHClient, error) {
	if keyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine home directory: %w", err)
		}
		keyPath = filepath.Join(home, ".ssh", "id_rsa")
	}

	// Expand ~ prefix.
	if strings.HasPrefix(keyPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to determine home directory: %w", err)
		}
		keyPath = filepath.Join(home, keyPath[2:])
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key %q: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key %q: %w", keyPath, err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // user-initiated remote connection
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	return &SSHClient{
		Host:   host,
		User:   user,
		Port:   port,
		Key:    keyPath,
		client: client,
	}, nil
}

// Run executes a command on the remote host and returns its combined output.
func (s *SSHClient) Run(cmd string) (string, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("remote command failed: %w\noutput: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// Close closes the underlying SSH connection.
func (s *SSHClient) Close() {
	if s.client != nil {
		if err := s.client.Close(); err != nil {
			return
		}
	}
}

// RunWithStdin executes a command on the remote host, writing stdin as input,
// and returns the combined output. Used to send shell scripts to remote Docker
// containers via: docker run --rm -i ... sh
func (s *SSHClient) RunWithStdin(cmd, stdin string) (string, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	session.Stdin = strings.NewReader(stdin)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("remote command failed: %w\noutput: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}
