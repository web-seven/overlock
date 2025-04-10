package configuration

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"

	"github.com/zalando/go-keyring"
	"go.uber.org/zap"
)

func decryptPackage(encryptedData string, key []byte) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("invalid ciphertext length")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func DecryptFromKeyring(encrypted string, logger *zap.SugaredLogger) (string, error) {
	service := "overlock"
	username := os.Getenv("USER")
	if username == "" {
		logger.Error("Environment variable USER is not set")
		return "", errors.New("missing USER environment variable")
	}

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
		return "", err
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		logger.Errorf("Failed to decode base64 string: %v", err)
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
