package eventwatcher

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/web-seven/overlock/internal/kube"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type ReconciliationCallback func(payload map[string]string)

func StartWatching(config *rest.Config, callback ReconciliationCallback) {
	clientset, err := kube.Client(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	listOptions := metav1.ListOptions{}
	ctx := context.Background()
	watcher, err := clientset.CoreV1().Events("").Watch(ctx, listOptions)
	if err != nil {
		log.Printf("[ERROR] Error starting event watcher: %v", err)
		return
	}
	defer watcher.Stop()

	log.Println("[INFO] Watching events for successful reconciliation actions...")
	for event := range watcher.ResultChan() {
		e, ok := event.Object.(*v1.Event)
		if !ok {
			log.Printf("[WARN] Received non-Event object type: %T", event.Object)
			continue
		}

		if isCrossplaneEvent(e) && isReconciliationSuccessful(e) {
			log.Printf("[INFO] Reconciliation success: %s - %s", e.InvolvedObject.Name, e.Reason)

			payload := map[string]string{
				"resource":  e.InvolvedObject.Name,
				"namespace": e.InvolvedObject.Namespace,
				"reason":    e.Reason,
				"message":   e.Message,
				"time":      e.LastTimestamp.Time.Format(time.RFC3339),
			}
			callback(payload)
		}
	}
	log.Println("[INFO] Event watcher channel closed.")
}

func isReconciliationSuccessful(e *v1.Event) bool {
	successfulReasons := []string{
		"SuccessfulCreate",
		"SuccessfulUpdate",
		"Started",
	}

	for _, reason := range successfulReasons {
		if e.Reason == reason {
			return true
		}
	}
	return false
}

func isCrossplaneEvent(e *v1.Event) bool {
	if strings.Contains(e.InvolvedObject.APIVersion, "crossplane.io") {
		return true
	}
	if strings.Contains(e.ReportingController, "crossplane") {
		return true
	}
	if strings.HasPrefix(e.InvolvedObject.Name, "composite") ||
		strings.HasPrefix(e.InvolvedObject.Name, "provider-") {
		return true
	}
	return false
}
