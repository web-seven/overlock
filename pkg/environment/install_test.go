package environment

import (
	"reflect"
	"testing"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/chart"
)

func TestCharts(t *testing.T) {
	disabled := false
	tests := map[string]struct {
		spec v1alpha1.EnvironmentSpec
		want []chart.Chart
	}{
		"DefaultsInstallAllTools": {
			spec: v1alpha1.EnvironmentSpec{},
			want: []chart.Chart{chart.CrossplaneChart{Args: []string{}}, chart.KyvernoChart{}, chart.CertManagerChart{}},
		},
		"DisabledToolsAreSkipped": {
			spec: v1alpha1.EnvironmentSpec{Tools: v1alpha1.ToolsSpec{
				Kyverno:     v1alpha1.ToolSpec{Enabled: &disabled},
				CertManager: v1alpha1.ToolSpec{Enabled: &disabled},
			}},
			want: []chart.Chart{chart.CrossplaneChart{Args: []string{}}},
		},
		"CrossplaneSettings": {
			spec: v1alpha1.EnvironmentSpec{
				Crossplane:     v1alpha1.CrossplaneSpec{MaxReconcileRate: 4, Args: []string{"--debug"}},
				Providers:      []string{"p"},
				Configurations: []string{"c"},
				Functions:      []string{"f"},
				Tools:          v1alpha1.ToolsSpec{Kyverno: v1alpha1.ToolSpec{Enabled: &disabled}},
			},
			want: []chart.Chart{
				chart.CrossplaneChart{
					Providers:      []string{"p"},
					Configurations: []string{"c"},
					Functions:      []string{"f"},
					Args:           []string{"--debug", "--max-reconcile-rate=4"},
				},
				chart.CertManagerChart{},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := Charts(tc.spec); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Charts() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
