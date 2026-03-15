package provider

import (
	"fmt"
	"log"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"

	"github.com/web-seven/overlock/plugins/cosmos/pkg/client"
)

func Register(importKeyName, importKeyPath, rpcURI, chainId, keyringBackend string, msg crossplanev1beta1.MsgCreateProvider) error {
	clientCtx, err := client.BuildClientContext(importKeyName, rpcURI, chainId, keyringBackend)
	if err != nil {
		return fmt.Errorf("failed to build client context: %w", err)
	}

	err = client.ImportKey(clientCtx, importKeyName, importKeyPath)
	if err != nil {
		return fmt.Errorf("failed to import key: %w", err)
	}
	log.Printf("Key %s imported successfully", importKeyName)

	info, err := clientCtx.Keyring.Key(importKeyName)
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}

	address, err := info.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get address: %w", err)
	}

	currentTime := time.Now()
	msg.RegisterTime = &currentTime
	msg.Creator = address.String()
	var sdkMsg sdk.Msg = &msg

	err = client.SendTxMessage(clientCtx, importKeyName, sdkMsg, chainId)
	if err != nil {
		return fmt.Errorf("failed to send tx: %w", err)
	}
	return nil
}
