package main

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// podReader returns the pod after failing the first failures Gets.
type podReader struct {
	failures int
	calls    int
	pod      corev1.Pod
}

func (r *podReader) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	r.calls++
	if r.calls <= r.failures {
		return errors.New("connection refused")
	}
	r.pod.DeepCopyInto(obj.(*corev1.Pod))
	return nil
}

func (r *podReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return errors.New("not implemented")
}

func operatorPod(image string) corev1.Pod {
	return corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "sidecar", Image: "other"},
		{Name: "manager", Image: image},
	}}}
}

func TestOwnImage(t *testing.T) {
	const image = "ghcr.io/web-seven/overlock-operator:test"

	tests := map[string]struct {
		inPod     bool
		failures  int
		wantImage string
		wantErr   bool
	}{
		"not in a pod":           {inPod: false, failures: 100},
		"API ready":              {inPod: true, wantImage: image},
		"API ready after errors": {inPod: true, failures: 3, wantImage: image},
		"API never ready":        {inPod: true, failures: 1000, wantErr: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.inPod {
				t.Setenv("POD_NAME", "overlock-operator-0")
				t.Setenv("POD_NAMESPACE", "overlock-system")
			} else {
				t.Setenv("POD_NAME", "")
				t.Setenv("POD_NAMESPACE", "")
			}
			reader := &podReader{failures: tc.failures, pod: operatorPod(image)}
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			got, err := ownImage(ctx, reader, time.Millisecond)
			if tc.wantErr != (err != nil) {
				t.Fatalf("ownImage() error = %v, want error %v", err, tc.wantErr)
			}
			if got != tc.wantImage {
				t.Errorf("ownImage() = %q, want %q", got, tc.wantImage)
			}
		})
	}
}
