package billing

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Watch(url string) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	fieldSelector := fields.AndSelectors(

		fields.OneTermEqualSelector("reason", "ReconcileSuccess"),
	)

	ctx := context.Background()
	watcher, err := clientset.CoreV1().Events("").Watch(ctx, metav1.ListOptions{

		FieldSelector: fieldSelector.String(),
	})
	if err != nil {

		log.Printf("Error watching events: %v", err)
		return
	}

	for event := range watcher.ResultChan() {
		if e, ok := event.Object.(*v1.Event); ok {
			log.Printf("[INFO] Received event: %s - %s/%s - %s\n",
				e.InvolvedObject.Kind,
				e.InvolvedObject.Namespace,
				e.InvolvedObject.Name,
				e.Reason,
			)

			payload := map[string]string{
				"provider": e.Source.Component,
				"action":   e.Reason,
				"resource": e.InvolvedObject.Name,
				"time":     e.LastTimestamp.String(),
			}
			jsonPayload, _ := json.MarshalIndent(payload, "", "  ")

			log.Printf("[DEBUG] Payload: %s\n", string(jsonPayload))

			// resp, err := http.Post(url, "application/json", bytes.NewReader(jsonPayload))
			// if err != nil {
			// 	log.Printf("[ERROR] Failed to POST event to %s: %v", url, err)
			// } else {
			// 	defer resp.Body.Close()
			// 	log.Printf("[INFO] Posted to %s - response: %s\n", url, resp.Status)
			// }
		}
	}

}
