package network

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/go-playground/validator/v10"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"
	storagev1beta1 "github.com/overlock-network/api/go/node/overlock/storage/v1beta1"
	"github.com/web-seven/overlock/plugins/cosmos/pkg/network/configuration"
	"github.com/web-seven/overlock/plugins/cosmos/pkg/types"

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

		queries := []string{
			"message.action='/overlock.crossplane.v1beta1.MsgCreateEnvironment'",
			"message.action='/overlock.storage.v1beta1.MsgCreateRegistry'",
			"message.action='/overlock.crossplane.v1beta1.MsgCreateConfiguration'",
		}
		if creator != "" {
			for i := range queries {
				queries[i] += fmt.Sprintf(" AND message.sender='%s'", creator)
			}
		}

		for _, query := range queries {
			subscribeMsg := fmt.Sprintf(`{"jsonrpc":"2.0","method":"subscribe","id":1,"params":{"query":"%s"}}`, query)
			if err = conn.WriteMessage(websocket.TextMessage, []byte(subscribeMsg)); err != nil {
				logger.Errorf("Subscription error: %v", err)
				conn.Close()
				time.Sleep(retryInterval)
				continue
			}
		}

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				logger.Errorf("Read error: %v", err)
				break
			}

			var envMsg crossplanev1beta1.MsgCreateEnvironment
			if decodedEnvMsg, err := processMessage(message, &envMsg, "/overlock.crossplane.v1beta1.MsgCreateEnvironment"); err == nil {
				logger.Infof("Received environment creation request: %s", decodedEnvMsg.Metadata.Name)
				go createEnvironment(engine, context.Background(), logger, envMsg)
				continue
			}

			var regMsg storagev1beta1.MsgCreateRegistry
			if decodedRegMsg, err := processMessage(message, &regMsg, "/overlock.storage.v1beta1.MsgCreateRegistry"); err == nil {
				logger.Infof("Received registry creation request: %s", decodedRegMsg.Name)
				go createRegistry(engine, context.Background(), logger, regMsg, client, config, grpcAddress)
			}

			var confMsg crossplanev1beta1.MsgCreateConfiguration
			if decodedConfMsg, err := processMessage(message, &confMsg, "/overlock.crossplane.v1beta1.MsgCreateConfiguration"); err == nil {
				logger.Infof("Received configuration creation request: %s", decodedConfMsg.Metadata.Name)
				go configuration.CreateConfiguration(context.Background(), logger, confMsg, config, dc)
				continue
			}
		}

		logger.Warn("Reconnecting to WebSocket in 3 seconds...")
		conn.Close()
		time.Sleep(retryInterval)
	}
}

func processMessage[T proto.Message](message []byte, msgStruct T, typeUrl string) (T, error) {
	var msgResponse types.MsgResponse

	err := json.Unmarshal(message, &msgResponse)
	if err != nil {
		log.Printf("JSON parse error: %v", err)
		return msgStruct, err
	}

	if err := ValidateRequest(msgResponse); err != nil {
		log.Printf("Validation error: %v", err)
		return msgStruct, err
	}

	txBytes, err := base64.StdEncoding.DecodeString(msgResponse.Result.Data.Value.TxResult.Tx)
	if err != nil {
		log.Printf("Failed to decode base64: %v", err)
		return msgStruct, err
	}

	var txMsg tx.Tx
	err = proto.Unmarshal(txBytes, &txMsg)
	if err != nil {
		log.Printf("Failed to unmarshal transaction: %v", err)
		return msgStruct, err
	}

	for _, msgAny := range txMsg.Body.Messages {
		if msgAny.TypeUrl == typeUrl {
			err := proto.Unmarshal(msgAny.Value, msgStruct)
			if err != nil {
				log.Printf("Failed to unmarshal %s: %v", typeUrl, err)
				return msgStruct, err
			}

			return msgStruct, nil
		}
	}

	return msgStruct, fmt.Errorf("no message found for TypeUrl: %s", typeUrl)
}

func ValidateRequest(i interface{}) error {
	validate := validator.New(validator.WithRequiredStructEnabled())

	if err := validate.Struct(i); err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return fmt.Errorf("invalid validation error: %w", err)
		}

		for _, err := range err.(validator.ValidationErrors) {
			return fmt.Errorf("validation error: field %s is invalid", err.Field())
		}
	}

	return nil
}
