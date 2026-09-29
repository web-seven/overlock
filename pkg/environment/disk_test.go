package environment

import (
	"testing"
)

func TestParseDfOutput(t *testing.T) {
	out := "Filesystem     1024-blocks      Used Available Capacity Mounted on\n" +
		"/dev/nvme0n1p2   479493808 419979700  35083672      93% /docker\n"
	got, err := parseDfOutput(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := diskUsage{Total: 479493808 * 1024, Available: 35083672 * 1024}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseDfOutputInvalid(t *testing.T) {
	for _, out := range []string{
		"",
		"Filesystem 1024-blocks Used Available Capacity Mounted on",
		"Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 abc 1 2 3% /docker",
	} {
		if _, err := parseDfOutput(out); err == nil {
			t.Errorf("expected error for %q", out)
		}
	}
}

func TestValidateDiskUsage(t *testing.T) {
	tests := []struct {
		name    string
		usage   diskUsage
		wantErr bool
	}{
		{"enough space", diskUsage{Total: 100, Available: 50}, false},
		{"at threshold", diskUsage{Total: 100, Available: 10}, false},
		{"low space", diskUsage{Total: 100, Available: 3}, true},
		{"unknown size", diskUsage{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDiskUsage(tt.usage); (err != nil) != tt.wantErr {
				t.Fatalf("validateDiskUsage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
