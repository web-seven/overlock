package provider

import (
	"log"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"
	"github.com/web-seven/overlock/plugins/cosmos/pkg/client"
)

func Register(importKeyName, importKeyPath, rpcURI, chainId, keyringBackend string, msg crossplanev1beta1.MsgCreateProvider) {

	clientCtx := client.BuildClientContext(importKeyName, rpcURI, chainId, keyringBackend)

	err := client.ImportKey(clientCtx, importKeyName, importKeyPath)
	if err != nil {
		log.Fatalf("Failed to import key: %v", err)
	} else {
		log.Printf("Key %s imported successfully", importKeyName)
	}

	info, err := clientCtx.Keyring.Key(importKeyName)
	if err != nil {
		log.Fatalf("Failed to get key: %v", err)
	}

	address, err := info.GetAddress()
	if err != nil {
		log.Fatalf("Failed to get address: %v", err)
	}

	currentTime := time.Now()
	msg.RegisterTime = &currentTime
	msg.Creator = address.String()
	var sdkMsg sdk.Msg = &msg

	err = client.SendTxMessage(clientCtx, importKeyName, sdkMsg, chainId)
	if err != nil {
		log.Fatalf("Failed to send tx: %v", err)
	}
}
