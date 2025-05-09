package client

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/input"
	tx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"
)

func BuildClientContext(from, rpcUri, chainId, keyringBackend string) client.Context {

	tempKeyringDir, err := os.MkdirTemp("", "overlock-keyring-*")
	if err != nil {
		log.Fatalf("Failed to create temporary keyring directory: %v", err)
	}

	defer os.RemoveAll(tempKeyringDir)

	encCfg := MakeEncodingConfig(auth.AppModuleBasic{},
		bank.AppModuleBasic{})
	crossplanev1beta1.RegisterInterfaces(encCfg.InterfaceRegistry)
	kr, err := keyring.New("", keyringBackend, tempKeyringDir, os.Stdin, encCfg.Codec)

	if err != nil {
		log.Fatalf("Failed to initialize keyring: %v", err)
	}

	clientRpc, err := client.NewClientFromNode(rpcUri)
	if err != nil {
		log.Fatalf("Failed to create RPC client: %v", err)
	}

	ctx := client.Context{}.
		WithFrom(from).
		WithBroadcastMode("sync").
		WithKeyring(kr).
		WithCodec(encCfg.Codec).
		WithTxConfig(encCfg.TxConfig).
		WithInterfaceRegistry(encCfg.InterfaceRegistry).
		WithClient(clientRpc).
		WithChainID(chainId)

	ctx = ctx.WithAccountRetriever(authtypes.AccountRetriever{})

	return ctx
}

func ImportKey(ctx client.Context, name string, keyFile string) error {
	checkName := func(name string) error {
		if name == "" {
			return fmt.Errorf("name cannot be empty")
		}
		return nil
	}
	if err := checkName(name); err != nil {
		return err
	}

	armor, err := os.ReadFile(keyFile)
	if err != nil {
		return err
	}

	buf := bufio.NewReader(ctx.Input)
	passphrase, err := input.GetPassword("Enter passphrase to decrypt your key:", buf)
	if err != nil {
		return err
	}

	return ctx.Keyring.ImportPrivKey(name, string(armor), passphrase)
}

func SendTxMessage(clientCtx client.Context, from string, msg sdk.Msg, chainId string) error {
	keyInfo, err := clientCtx.Keyring.Key(from)
	if err != nil {
		return fmt.Errorf("failed to retrieve key info: %w", err)
	}
	address, err := keyInfo.GetAddress()
	if err != nil {
		return fmt.Errorf("failed to get address from key info: %w", err)
	}

	clientCtx = clientCtx.WithFromAddress(address).WithFromName(from)

	if clientCtx.TxConfig == nil {
		return fmt.Errorf("TxConfig is nil — you must set it in client.Context")
	}
	accountRetrievers := authtypes.AccountRetriever{}
	fromAddress := clientCtx.GetFromAddress()
	if fromAddress.Empty() {
		return fmt.Errorf("from address is empty")
	}
	fmt.Println("From address:", fromAddress.String())
	account, err := accountRetrievers.GetAccount(clientCtx, fromAddress)
	if err != nil {
		return fmt.Errorf("failed to retrieve account: %w", err)
	}

	txf := tx.Factory{}.
		WithChainID(chainId).
		WithTxConfig(clientCtx.TxConfig).
		WithKeybase(clientCtx.Keyring).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT).
		WithAccountRetriever(accountRetrievers).
		WithAccountNumber(account.GetAccountNumber()).
		WithSequence(account.GetSequence()).
		WithGas(200000)

	if err := tx.BroadcastTx(clientCtx, txf, msg); err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return nil
}
