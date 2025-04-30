package billing

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"

	eventwatcher "github.com/web-seven/overlock/pkg/billing"
	"k8s.io/client-go/rest"
)

func Watch(mockURL string, config *rest.Config) {
	eventwatcher.StartWatching(config, func(payload map[string]string) {
		sendPostRequest(mockURL, payload)
	})
}

func sendPostRequest(url string, payload map[string]string) {
	if url == "" {
		log.Printf("[ERROR] URL is empty, cannot send POST request")
		return
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal payload: %v", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[ERROR] POST request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[INFO] POST request sent to %s - status: %s", url, resp.Status)
}
