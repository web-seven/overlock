package network

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	crossplanev1beta1 "github.com/web-seven/overlock-api/go/node/overlock/crossplane/v1beta1"
	"github.com/web-seven/overlock/plugins/solana/pkg/types"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func Subscribe(engine, creator, host, port, path, grpcAddress string, client *kubernetes.Clientset, config *rest.Config, dc *dynamic.DynamicClient) {
	logger := zap.NewExample().Sugar()
	defer logger.Sync()

	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%s", host, port), Path: path}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	go func() {
		<-quit
		logger.Info("Shutting down WebSocket listener...")
		cancel()
	}()

	retryInterval := 3 * time.Second

	for {
		select {
		case <-ctx.Done():
			logger.Info("WebSocket listener stopped.")
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			logger.Errorf("WebSocket connection failed: %v", err)
			time.Sleep(retryInterval)
			continue
		}

		logger.Info("Connected to WebSocket")

		var filter interface{}
		if creator != "" {
			filter = map[string]interface{}{
				"mentions": []string{creator},
			}
		} else {
			filter = map[string]interface{}{
				"all": nil,
			}
		}

		subscribeRequest := types.LogsSubscribeRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "logsSubscribe",
			Params: []interface{}{
				filter,
				map[string]string{"commitment": "finalized"},
			},
		}

		subscribeMsgBytes, err := json.Marshal(subscribeRequest)
		if err != nil {
			logger.Errorf("Failed to marshal subscribe request: %v", err)
			conn.Close()
			time.Sleep(retryInterval)
			continue
		}

		if err = conn.WriteMessage(websocket.TextMessage, []byte(subscribeMsgBytes)); err != nil {
			logger.Errorf("Subscription error: %v", err)
			conn.Close()
			time.Sleep(retryInterval)
			continue
		}

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				logger.Errorf("Read error: %v", err)
				break
			}

			var notif types.LogsNotification
			if err := json.Unmarshal(message, &notif); err != nil {
				logger.Errorf("Failed to parse notification: %v", err)
				continue
			}

			for _, log := range notif.Params.Result.Value.Logs {
				if strings.HasPrefix(log, "Program log: Environment base64: ") {
					base64Data := strings.TrimPrefix(log, "Program log: Environment base64: ")
					rawData, err := base64.StdEncoding.DecodeString(base64Data)
					if err != nil {
						logger.Errorf("Failed to decode base64: %v", err)
						continue
					}

					var envMsg crossplanev1beta1.Environment
					if err := proto.Unmarshal(rawData, &envMsg); err != nil {
						logger.Errorf("Failed to unmarshal Protobuf: %v", err)
						continue
					}

					go createEnvironment(engine, context.Background(), logger, envMsg)
					continue
				}
			}
		}
	}
}
