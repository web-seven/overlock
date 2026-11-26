package environment

import (
	"testing"

	"github.com/web-seven/overlock/api/v1alpha1"
)

func TestEnvironmentSpecEngineScheduling(t *testing.T) {
	spec := New("k3s-docker", "dev").WithProviders([]string{"p"}).environmentSpec()
	if spec.Scheduling == nil || spec.Scheduling.NodeSelector[scopeLabel] != scopeEngine {
		t.Fatalf("k3s-docker scheduling = %+v", spec.Scheduling)
	}
	if len(spec.Providers) != 1 {
		t.Errorf("providers lost: %v", spec.Providers)
	}

	custom := &v1alpha1.SchedulingSpec{NodeSelector: map[string]string{"pool": "infra"}}
	spec = New("k3s-docker", "dev").WithSpec(v1alpha1.EnvironmentSpec{Scheduling: custom}).environmentSpec()
	if spec.Scheduling.NodeSelector["pool"] != "infra" || len(spec.Scheduling.NodeSelector) != 1 {
		t.Errorf("explicit scheduling overridden: %+v", spec.Scheduling)
	}

	if spec := New("kind", "dev").environmentSpec(); spec.Scheduling != nil {
		t.Errorf("kind scheduling = %+v, want nil", spec.Scheduling)
	}
}

func TestEnvironmentSpecImageStoreDefault(t *testing.T) {
	if spec := New("k3s-docker", "dev").environmentSpec(); !spec.ImageStoreEnabled() {
		t.Error("k3s-docker environments must use the image store by default")
	}
	if spec := New("kind", "dev").environmentSpec(); spec.ImageStoreEnabled() {
		t.Error("kind environments must keep the Docker registry")
	}
	off := v1alpha1.EnvironmentSpec{ImageStore: &v1alpha1.ImageStoreSpec{Enabled: false}}
	if spec := New("k3s-docker", "dev").WithSpec(off).environmentSpec(); spec.ImageStoreEnabled() {
		t.Error("an explicitly disabled image store was enabled")
	}
}
