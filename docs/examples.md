# Usage Examples

This document provides practical examples and common workflows for using Overlock.

## Table of Contents

- [Quick Start](#quick-start)
- [Basic Development Setup](#basic-development-setup)
- [Cloud Provider Workflows](#cloud-provider-workflows)
  - [Working with GCP](#working-with-gcp)
  - [Working with AWS](#working-with-aws)
  - [Working with Azure](#working-with-azure)
- [Local Package Development](#local-package-development)
- [Multi-Environment Workflow](#multi-environment-workflow)
- [CI/CD Integration](#cicd-integration)

## Quick Start

Get up and running with Overlock in under 5 minutes:

```bash
# 1. Create a new Crossplane environment
overlock environment create my-dev-env

# 2. Install a cloud provider (GCP example)
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0

# 3. Apply a configuration
overlock configuration apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31

# 4. List your environments
overlock environment list

# 5. Check installed providers
overlock provider list
```

## Basic Development Setup

Set up a complete development environment with commonly used tools:

```bash
# Create development environment
overlock environment create crossplane-dev

# Set up local registry for testing packages
overlock registry create --local --default

# Install commonly used providers
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-kubernetes:v0.14.0

# Apply base configurations
overlock configuration apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31

# Verify setup
overlock provider list
overlock configuration list
```

## Cloud Provider Workflows

### Working with GCP

Complete workflow for managing GCP infrastructure with Crossplane:

```bash
# 1. Create GCP-focused environment
overlock environment create gcp-project

# 2. Install GCP provider
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0

# 3. Install useful configurations
overlock configuration apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31

# 4. Wait for provider to be ready
kubectl wait --for=condition=healthy provider.pkg.crossplane.io/provider-gcp --timeout=300s

# 5. Configure GCP credentials
# Create a service account key and configure it
kubectl create secret generic gcp-creds \
  -n crossplane-system \
  --from-file=creds=./gcp-credentials.json

# 6. Create ProviderConfig
cat <<EOF | kubectl apply -f -
apiVersion: gcp.upbound.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  projectID: your-gcp-project-id
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: gcp-creds
      key: creds
EOF

# 7. Apply your infrastructure definitions
overlock resource apply ./infrastructure.yaml

# 8. Monitor resource creation
kubectl get managed
```

### Working with AWS

Set up AWS infrastructure management:

```bash
# Create AWS environment
overlock environment create aws-project

# Install AWS provider family
overlock provider install xpkg.upbound.io/upbound/provider-aws-ec2:v1.0.0
overlock provider install xpkg.upbound.io/upbound/provider-aws-s3:v1.0.0
overlock provider install xpkg.upbound.io/upbound/provider-aws-rds:v1.0.0

# Configure AWS credentials
kubectl create secret generic aws-creds \
  -n crossplane-system \
  --from-literal=credentials="[default]
aws_access_key_id = YOUR_ACCESS_KEY
aws_secret_access_key = YOUR_SECRET_KEY"

# Create ProviderConfig
cat <<EOF | kubectl apply -f -
apiVersion: aws.upbound.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: aws-creds
      key: credentials
EOF

# Apply AWS resources
overlock resource apply ./aws-infrastructure.yaml
```

### Working with Azure

Azure infrastructure setup:

```bash
# Create Azure environment
overlock environment create azure-project

# Install Azure provider
overlock provider install xpkg.upbound.io/upbound/provider-azure:v0.36.0

# Configure Azure credentials
kubectl create secret generic azure-creds \
  -n crossplane-system \
  --from-literal=credentials='{"clientId": "YOUR_CLIENT_ID",
"clientSecret": "YOUR_CLIENT_SECRET",
"subscriptionId": "YOUR_SUBSCRIPTION_ID",
"tenantId": "YOUR_TENANT_ID"}'

# Apply Azure resources
overlock resource apply ./azure-infrastructure.yaml
```

## Local Package Development

Develop and test Crossplane packages locally with live reload:

### Configuration Development

```bash
# 1. Create development environment
overlock environment create package-dev

# 2. Set up local registry
overlock registry create --local --default

# 3. Start configuration development server
# This watches for changes and automatically rebuilds
overlock configuration serve ./my-config-package

# In another terminal:
# 4. Test your configuration
cat <<EOF | kubectl apply -f -
apiVersion: example.com/v1alpha1
kind: MyResource
metadata:
  name: test-resource
spec:
  parameter: value
EOF

# 5. Watch resources being created
kubectl get composite
kubectl get managed

# 6. Make changes to your configuration
# The serve command will automatically reload

# 7. Test changes immediately
kubectl delete myresource test-resource
kubectl apply -f test-resource.yaml
```

### Provider Development

```bash
# Create provider development environment
overlock environment create provider-dev

# Start provider development server
# Watches code and rebuilds on changes
overlock provider serve ./my-provider ./cmd/provider

# In another terminal, test your provider
kubectl apply -f test-managed-resource.yaml

# Check provider logs
kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=my-provider -f
```

### Function Development

```bash
# Create function development environment
overlock environment create function-dev

# Start function server
overlock function serve ./my-function

# Test function in a composition
kubectl apply -f test-composition.yaml
kubectl apply -f test-claim.yaml

# Monitor function execution
kubectl logs -n crossplane-system -l pkg.crossplane.io/function=my-function -f
```

## Multi-Environment Workflow

Manage multiple isolated environments for different purposes:

```bash
# Development environment
overlock environment create dev
overlock environment start dev

# Install development tools
overlock --namespace dev-ns provider install <provider>

# Do development work...
# Code, test, iterate

# Stop when done to save resources
overlock environment stop dev

# Testing environment
overlock environment create test
overlock environment start test

# Install same providers for testing
overlock --namespace test-ns provider install <provider>

# Run integration tests
overlock resource apply ./test-manifests/

# Verify tests pass
kubectl get managed -n test-ns

# Stop test environment
overlock environment stop test

# Staging environment
overlock environment create staging
overlock --namespace staging-ns provider install <provider>

# Deploy to staging
overlock resource apply ./staging-manifests/

# Production-like environment
overlock environment create prod-like
# ... configure with production-similar settings
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Test Crossplane Configuration

on:
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install Overlock
        run: |
          curl -sL "https://raw.githubusercontent.com/web-seven/overlock/refs/heads/main/scripts/install.sh" | sh
          sudo mv overlock /usr/local/bin/

      - name: Create test environment
        run: overlock environment create ci-test

      - name: Install providers
        run: |
          overlock provider install xpkg.upbound.io/crossplane-contrib/provider-kubernetes:v0.14.0

      - name: Test configuration
        run: |
          overlock configuration serve ./packages/my-config &
          sleep 30
          kubectl apply -f test/fixtures/
          kubectl wait --for=condition=ready --timeout=300s composite --all

      - name: Cleanup
        if: always()
        run: overlock environment delete ci-test
```

### GitLab CI Example

```yaml
test-configuration:
  stage: test
  image: ubuntu:22.04
  before_script:
    - apt-get update && apt-get install -y curl docker.io
    - curl -sL "https://raw.githubusercontent.com/web-seven/overlock/refs/heads/main/scripts/install.sh" | sh
    - mv overlock /usr/local/bin/
  script:
    - overlock environment create ci-test
    - overlock provider install xpkg.upbound.io/crossplane-contrib/provider-kubernetes:v0.14.0
    - overlock configuration apply ./packages/my-config
    - kubectl apply -f test/
    - kubectl wait --for=condition=ready --timeout=300s composite --all
  after_script:
    - overlock environment delete ci-test
```

## Advanced Patterns

### Package Publishing Workflow

```bash
# 1. Develop locally
overlock environment create package-dev
overlock configuration serve ./my-package

# 2. Test thoroughly
kubectl apply -f test/

# 3. Build and push to registry
# (assuming you have crossplane CLI)
crossplane build configuration
crossplane push configuration registry.example.com/my-package:v1.0.0

# 4. Test published package
overlock environment create package-test
overlock configuration apply registry.example.com/my-package:v1.0.0

# 5. Verify it works
kubectl apply -f examples/
```

### Debugging Resources

```bash
# Check all Crossplane resources
kubectl get crossplane

# Describe a composite resource
kubectl describe composite my-resource

# Check events
kubectl get events --sort-by='.lastTimestamp'

# View provider logs
kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=provider-gcp

# Enable detailed logging
kubectl set env deployment/crossplane -n crossplane-system --containers=crossplane DEBUG=true
```

## Additional Resources

- [Command Reference](commands.md) - Detailed command documentation
- [Configuration Guide](configuration.md) - Configuration options
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
