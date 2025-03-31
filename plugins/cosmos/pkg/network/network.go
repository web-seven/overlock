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
	"github.com/web-seven/overlock-api/go/node/overlock/crossplane/v1beta1"
	"github.com/web-seven/overlock/pkg/environment"
	"github.com/web-seven/overlock/plugins/cosmos/pkg/types"

	"go.uber.org/zap"
)

func Subscribe(engine, creator, host, port, path string) {
	logger := zap.NewExample().Sugar()
	defer logger.Sync()

	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%s", host, port), Path: path}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	for {
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			logger.Errorf("WebSocket connection failed: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		logger.Info("Connected to WebSocket")
		query := "message.action='/overlock.crossplane.v1beta1.MsgCreateEnvironment'"
		if creator != "" {
			query += fmt.Sprintf(" AND message.sender='%s'", creator)
		}

		subscribeMsg := fmt.Sprintf(`{"jsonrpc":"2.0","method":"subscribe","id":1,"params":{"query":"%s"}}`, query)
		err = conn.WriteMessage(websocket.TextMessage, []byte(subscribeMsg))
		if err != nil {
			logger.Errorf("Subscription error: %v", err)
			conn.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		go func() {
			<-quit
			logger.Info("Shutting down WebSocket listener...")
			conn.Close()
			os.Exit(0)
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				logger.Errorf("Read error: %v", err)
				break
			}

			var msg v1beta1.MsgCreateEnvironment
			decodedMsg, err := processMessage(message, &msg, "/overlock.crossplane.v1beta1.MsgCreateEnvironment")
			if err != nil {
				logger.Errorf("Error processing message: %v", err)
				continue
			}

			logger.Infof("Received environment creation request: %s", decodedMsg.Metadata.Name)

			createEnvironment(engine, context.Background(), logger, msg)
		}

		logger.Warn("Reconnecting to WebSocket in 3 seconds...")
		conn.Close()
		time.Sleep(3 * time.Second)
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

func createEnvironment(engine string, ctx context.Context, logger *zap.SugaredLogger, msg v1beta1.MsgCreateEnvironment) {

	err := environment.New(engine, msg.Metadata.Name).Create(ctx, logger)
	if err != nil {
		logger.Errorf("Error creating environment: %v", err)
		return
	}

	logger.Infof("Successfully created environment: %s", msg.Metadata.Name)
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
