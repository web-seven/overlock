package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"
)

func Register() {
	endpoint := rpc.TestNet_RPC
	client := rpc.New(endpoint)

	parseUint32 := func(s string) uint32 {
		val, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return 0
		}
		return uint32(val)
	}

	providerMetadata := crossplanev1beta1.Metadata{
		Name: "name",
	}

	provider := crossplanev1beta1.MsgCreateProvider{
		Metadata:        &providerMetadata,
		Ip:              "0.0.0.0",
		Port:            parseUint32("8900"),
		CountryCode:     "MD",
		EnvironmentType: "test",
		Availability:    "available",
	}
	providerBytes, err := provider.Marshal()
	if err != nil {
		panic(err)
	}
	prikey, err := solana.PrivateKeyFromSolanaKeygenFile("~/.config/solana/id.json")
	if err != nil {
		panic(err)
	}
	pubkey := prikey.PublicKey()
	fmt.Println("Public key: ", pubkey.String())

	instruction := solana.NewInstruction(
		pubkey,
		solana.AccountMetaSlice{},
		providerBytes,
	)

	builder := solana.NewTransactionBuilder().AddInstruction(instruction).SetFeePayer(pubkey)
	transaction, err := builder.Build()
	if err != nil {
		panic(err)
	}
	fmt.Println("Transaction is signed: ", transaction.IsSigner(pubkey))

	sig, err := client.SendTransaction(context.TODO(), transaction)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx signature: ", sig.String())

}
