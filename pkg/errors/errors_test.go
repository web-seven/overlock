package errors

import (
	"errors"
	"testing"
)

func TestInvalidConfigError(t *testing.T) {
	// Test basic InvalidConfigError
	err := NewInvalidConfigError("timeout", "invalid", "must be a positive number")
	expected := "invalid configuration: field 'timeout' with value 'invalid': must be a positive number"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}

	// Test InvalidConfigError with cause
	cause := errors.New("parsing error")
	err = NewInvalidConfigErrorWithCause("port", "abc", "must be numeric", cause)
	if !errors.Is(err.Unwrap(), cause) {
		t.Error("Expected unwrapped error to be the cause")
	}

	// Test error type checking
	if !IsInvalidConfigError(err) {
		t.Error("Expected IsInvalidConfigError to return true")
	}
}

func TestKubernetesConnectionError(t *testing.T) {
	// Test basic KubernetesConnectionError
	err := NewKubernetesConnectionError("minikube", "localhost:8443", "connection refused")
	expected := "kubernetes connection error: context 'minikube' (host: localhost:8443): connection refused"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}

	// Test KubernetesConnectionError with cause
	cause := errors.New("network unreachable")
	err = NewKubernetesConnectionErrorWithCause("prod", "prod-cluster.example.com", "timeout", cause)
	if !errors.Is(err.Unwrap(), cause) {
		t.Error("Expected unwrapped error to be the cause")
	}

	// Test error type checking
	if !IsKubernetesConnectionError(err) {
		t.Error("Expected IsKubernetesConnectionError to return true")
	}
}

func TestPackageNotFoundError(t *testing.T) {
	// Test basic PackageNotFoundError
	err := NewPackageNotFoundError("crossplane-provider-aws", "ghcr.io", "v1.0.0", "version does not exist")
	expected := "package not found: 'crossplane-provider-aws' version 'v1.0.0' in registry 'ghcr.io': version does not exist"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}

	// Test PackageNotFoundError with cause
	cause := errors.New("HTTP 404")
	err = NewPackageNotFoundErrorWithCause("my-package", "registry.local", "", "package removed", cause)
	if !errors.Is(err.Unwrap(), cause) {
		t.Error("Expected unwrapped error to be the cause")
	}

	// Test error type checking
	if !IsPackageNotFoundError(err) {
		t.Error("Expected IsPackageNotFoundError to return true")
	}
}

func TestErrorTypeDiscrimination(t *testing.T) {
	configErr := NewInvalidConfigError("field", "value", "message")
	k8sErr := NewKubernetesConnectionError("context", "host", "message")
	packageErr := NewPackageNotFoundError("package", "registry", "version", "message")

	// Test that each type only matches its own checker
	if IsKubernetesConnectionError(configErr) || IsPackageNotFoundError(configErr) {
		t.Error("InvalidConfigError should not match other error types")
	}

	if IsInvalidConfigError(k8sErr) || IsPackageNotFoundError(k8sErr) {
		t.Error("KubernetesConnectionError should not match other error types")
	}

	if IsInvalidConfigError(packageErr) || IsKubernetesConnectionError(packageErr) {
		t.Error("PackageNotFoundError should not match other error types")
	}

	// Test with regular error
	regularErr := errors.New("regular error")
	if IsInvalidConfigError(regularErr) || IsKubernetesConnectionError(regularErr) || IsPackageNotFoundError(regularErr) {
		t.Error("Regular error should not match any custom error types")
	}
}
