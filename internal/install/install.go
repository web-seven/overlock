// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import "helm.sh/helm/v3/pkg/chart"

// InstallOption customizes the behavior of an install.
type InstallOption func(*chart.Chart) error

// UpgradeOption customizes the behavior of an upgrade.
type UpgradeOption func(oldVersion string, ch *chart.Chart) error

// Manager can install and manage Upbound software in a Kubernetes cluster.
// TODO(hasheddan): support custom error types, such as AlreadyExists.
type Manager interface {
	GetCurrentVersion() (string, error)
	Install(version string, parameters map[string]any, opts ...InstallOption) error
	Upgrade(version string, parameters map[string]any, opts ...UpgradeOption) error
	Uninstall() error
}

// ParameterParser parses install and upgrade parameters.
type ParameterParser interface {
	Parse() (map[string]any, error)
}
