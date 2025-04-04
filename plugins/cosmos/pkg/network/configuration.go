package network

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"

	crossplanev1beta1 "github.com/web-seven/overlock-api/go/node/overlock/crossplane/v1beta1"
	"github.com/web-seven/overlock/pkg/configuration"
	"github.com/zalando/go-keyring"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

func createConfiguration(ctx context.Context, logger *zap.SugaredLogger, msg crossplanev1beta1.MsgCreateConfiguration, config *rest.Config) {
	configurationPackage := msg.Spec.GetPackage()
	if configurationPackage == "" {
		logger.Error("Configuration package is nil")
		return
	}
	pkg, err := decryptPackage(configurationPackage, logger)
	if err != nil {
		logger.Errorf("Failed to decode package: %v", err)
		return
	}
	cfg := configuration.New(pkg)
	cfg.Apply(ctx, config, logger)

	logger.Infof("Successfully created configuration: %s", msg.Metadata.Name)
}

func decryptPackage(encryptedPkg string, logger *zap.SugaredLogger) (string, error) {
	service := "overlock"
	username := os.Getenv("USER")
	privateKeyStr, err := keyring.Get(service, username)
	if err != nil {
		logger.Errorf("Key not found in keyring: %v", err)
		return "", err
	}

	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		logger.Error("Failed to parse private key PEM")
		return "", errors.New("failed to parse private key PEM")
	}

	privateKey, err := parseKey(block.Bytes, logger)
	if err != nil {
		logger.Errorf("Failed to parse private key: %v", err)
		return "", err
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(encryptedPkg)
	if err != nil {
		logger.Errorf("Failed to decode base64 package: %v", err)
		return "", err
	}

	decryptedBytes, err := rsa.DecryptOAEP(crypto.SHA256.New(), rand.Reader, privateKey, decodedBytes, nil)
	if err != nil {
		logger.Errorf("RSA decryption failed: %v", err)
		return "", err
	}

	return string(decryptedBytes), nil
}

func parseKey(keyBytes []byte, logger *zap.SugaredLogger) (*rsa.PrivateKey, error) {
	if key, err := x509.ParsePKCS8PrivateKey(keyBytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		logger.Error("Parsed key is not an RSA private key")
		return nil, errors.New("parsed key is not an RSA private key")
	}

	if key, err := x509.ParsePKCS1PrivateKey(keyBytes); err == nil {
		return key, nil
	}

	logger.Error("Failed to parse private key")
	return nil, errors.New("failed to parse private key")
}
